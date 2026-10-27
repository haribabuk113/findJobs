package profiles

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strconv"
	"strings"
)

// SkillRef is one profile skill as a taxonomy term.
type SkillRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// SkillSet is the candidate's skills for the lightweight sync (manual:
// "Lightweight Profile Skills Synchronization Pipeline"). Skills is omitted
// when the client's signature already matches.
type SkillSet struct {
	Signature string     `json:"signature"`
	Changed   bool       `json:"changed"`
	Skills    []SkillRef `json:"skills,omitempty"`
}

// SkillSignature fingerprints a skill set: count plus sorted ids, hashed.
// Order and group changes don't affect matching, so they don't change it;
// a merge repoints ids, so it does.
func SkillSignature(skills []SkillRef) string {
	ids := make([]int64, len(skills))
	for i, s := range skills {
		ids[i] = s.ID
	}
	slices.Sort(ids)
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.FormatInt(id, 10)
	}
	sum := sha256.Sum256([]byte(strconv.Itoa(len(ids)) + ":" + strings.Join(parts, ",")))
	return hex.EncodeToString(sum[:8])
}

// NewSkillSet builds the full sync response for skills.
func NewSkillSet(skills []SkillRef) *SkillSet {
	if skills == nil {
		skills = []SkillRef{}
	}
	return &SkillSet{Signature: SkillSignature(skills), Changed: true, Skills: skills}
}

// ErrNoProfile: one-click skill changes need a saved profile to attach to.
var ErrNoProfile = &NotFoundError{Message: "save your profile first"}

// ErrUnknownSkill: the skill id is not in the taxonomy.
var ErrUnknownSkill = &ValidationError{Field: "skill_id", Message: "unknown skill"}

// ErrTooManySkills: the profile already has MaxSkills.
var ErrTooManySkills = &ValidationError{Field: "skills", Message: "you can add at most " + strconv.Itoa(MaxSkills) + " skills"}
