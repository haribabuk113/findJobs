package inbound

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	apppayments "findJobs/internal/application/payments"
	domainpayments "findJobs/internal/domain/payments"

	"github.com/gin-gonic/gin"
)

// PaymentsHandler handles wallets, top-ups and the gateway webhook.
type PaymentsHandler struct {
	service apppayments.PaymentService
}

// NewPaymentsHandler creates the HTTP handler for payments.
func NewPaymentsHandler(service apppayments.PaymentService) *PaymentsHandler {
	return &PaymentsHandler{service: service}
}

// RegisterPaymentRoutes registers the wallet routes and the (unauthenticated,
// signature-checked) gateway webhook on api.
func RegisterPaymentRoutes(api *gin.RouterGroup, h *PaymentsHandler, requireUser gin.HandlerFunc) {
	api.GET("/wallet", requireUser, h.Wallet)
	api.POST("/wallet/topups", requireUser, h.Topup)
	api.GET("/companies/:slug/wallet", requireUser, h.CompanyWallet)
	api.POST("/companies/:slug/wallet/topups", requireUser, h.CompanyTopup)
	api.POST("/payments/webhook", h.Webhook)
}

type topupRequest struct {
	AmountPaise int64 `json:"amount_paise"`
}

// Wallet handles GET /wallet?page=&page_size=.
//
//	@Summary	My wallet: balance and ledger, newest first
//	@ID			getMyWallet
//	@Tags		wallet
//	@Produce	json
//	@Security	sessionCookie
//	@Param		page		query	int	false	"Page (default 1)"
//	@Param		page_size	query	int	false	"Page size (default 20, max 50)"
//	@Success	200	{object}	adapters.Response{data=domainpayments.Wallet}
//	@Failure	400,401,500	{object}	adapters.Response
//	@Router		/api/v1/wallet [get]
func (h *PaymentsHandler) Wallet(ctx *gin.Context) {
	h.wallet(ctx, domainpayments.Owner{UserID: ctx.GetString(userIDKey)})
}

// CompanyWallet handles GET /companies/{slug}/wallet?page=&page_size= (company admins).
// With format=csv it downloads every transaction instead (paging is ignored).
//
//	@Summary	Company wallet: balance and transactions, newest first (format=csv: full history as CSV)
//	@ID			getCompanyWallet
//	@Tags		wallet
//	@Produce	json,text/csv
//	@Security	sessionCookie
//	@Param		slug		path	string	true	"Company slug"
//	@Param		page		query	int		false	"Page (default 1)"
//	@Param		page_size	query	int		false	"Page size (default 20, max 50)"
//	@Param		format		query	string	false	"csv downloads the full transaction history"	Enums(csv)
//	@Success	200	{object}	adapters.Response{data=domainpayments.Wallet}
//	@Failure	400,401,403,404,500	{object}	adapters.Response
//	@Router		/api/v1/companies/{slug}/wallet [get]
func (h *PaymentsHandler) CompanyWallet(ctx *gin.Context) {
	owner := domainpayments.Owner{UserID: ctx.GetString(userIDKey), CompanySlug: ctx.Param("slug")}
	switch ctx.Query("format") {
	case "":
		h.wallet(ctx, owner)
	case "csv":
		h.walletCSV(ctx, owner)
	default:
		fail(ctx, http.StatusBadRequest, fmt.Errorf("format must be csv"))
	}
}

func (h *PaymentsHandler) walletCSV(ctx *gin.Context, owner domainpayments.Owner) {
	w, err := h.service.Ledger(ctx, owner)
	if err != nil {
		failErr(ctx, err)
		return
	}
	// The slug is safe in the header: the lookup above matched it to a stored slug.
	ctx.Header("Cache-Control", "private, no-store")
	ctx.Header("Content-Type", "text/csv; charset=utf-8")
	ctx.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="wallet-%s-%s.csv"`, owner.CompanySlug, time.Now().Format("2006-01-02")))
	ctx.Status(http.StatusOK)
	if err := domainpayments.WriteCSV(ctx.Writer, w.Entries); err != nil {
		ctx.Error(err)
	}
}

func (h *PaymentsHandler) wallet(ctx *gin.Context, owner domainpayments.Owner) {
	page, err1 := queryInt(ctx, "page")
	size, err2 := queryInt(ctx, "page_size")
	if err1 != nil || err2 != nil {
		fail(ctx, http.StatusBadRequest, fmt.Errorf("page and page_size must be numbers"))
		return
	}
	result, err := h.service.Wallet(ctx, owner, page, size)
	respond(ctx, http.StatusOK, "Wallet", result, err)
}

func queryInt(ctx *gin.Context, key string) (int, error) {
	if v := ctx.Query(key); v != "" {
		return strconv.Atoi(v)
	}
	return 0, nil
}

// Topup handles POST /wallet/topups {"amount_paise"}.
//
//	@Summary	Start a personal wallet top-up (returns the checkout order; credited by the webhook)
//	@ID			topupMyWallet
//	@Tags		wallet
//	@Accept		json
//	@Produce	json
//	@Security	sessionCookie
//	@Param		body	body	topupRequest	true	"Amount in paise (100 to 50000000)"
//	@Success	201	{object}	adapters.Response{data=domainpayments.Checkout}
//	@Failure	400,401,500,503	{object}	adapters.Response
//	@Router		/api/v1/wallet/topups [post]
func (h *PaymentsHandler) Topup(ctx *gin.Context) {
	h.topup(ctx, domainpayments.Owner{UserID: ctx.GetString(userIDKey)})
}

// CompanyTopup handles POST /companies/{slug}/wallet/topups {"amount_paise"} (company admins).
//
//	@Summary	Start a company wallet top-up (returns the checkout order; credited by the webhook)
//	@ID			topupCompanyWallet
//	@Tags		wallet
//	@Accept		json
//	@Produce	json
//	@Security	sessionCookie
//	@Param		slug	path	string			true	"Company slug"
//	@Param		body	body	topupRequest	true	"Amount in paise (100 to 50000000)"
//	@Success	201	{object}	adapters.Response{data=domainpayments.Checkout}
//	@Failure	400,401,403,404,500,503	{object}	adapters.Response
//	@Router		/api/v1/companies/{slug}/wallet/topups [post]
func (h *PaymentsHandler) CompanyTopup(ctx *gin.Context) {
	h.topup(ctx, domainpayments.Owner{UserID: ctx.GetString(userIDKey), CompanySlug: ctx.Param("slug")})
}

func (h *PaymentsHandler) topup(ctx *gin.Context, owner domainpayments.Owner) {
	var in topupRequest
	if err := ctx.ShouldBindJSON(&in); err != nil {
		fail(ctx, http.StatusBadRequest, fmt.Errorf("give amount_paise"))
		return
	}
	result, err := h.service.Topup(ctx, owner, in.AmountPaise)
	respond(ctx, http.StatusCreated, "Top-up started", result, err)
}

// Webhook handles POST /payments/webhook from the gateway. 2xx tells the
// gateway to stop retrying, so only failures a retry can fix return 5xx.
//
//	@Summary	Payment gateway webhook (Razorpay order.paid; X-Razorpay-Signature required)
//	@ID			paymentWebhook
//	@Tags		wallet
//	@Accept		json
//	@Produce	json
//	@Param		X-Razorpay-Signature	header	string	true	"HMAC-SHA256 of the raw body"
//	@Success	200	{object}	adapters.Response
//	@Failure	400,500,503	{object}	adapters.Response
//	@Router		/api/v1/payments/webhook [post]
func (h *PaymentsHandler) Webhook(ctx *gin.Context) {
	body, err := ctx.GetRawData()
	if err != nil {
		fail(ctx, http.StatusBadRequest, fmt.Errorf("unreadable body"))
		return
	}
	err = h.service.HandleWebhook(ctx, body, ctx.GetHeader("X-Razorpay-Signature"))
	respond(ctx, http.StatusOK, "Webhook processed", nil, err)
}
