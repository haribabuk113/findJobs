package common

func MapSlice[S, D any](in []S, f func(S) D) []D {
	out := make([]D, 0, len(in))
	for _, v := range in {
		out = append(out, f(v))
	}
	return out
}
