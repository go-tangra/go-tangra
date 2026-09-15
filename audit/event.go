package audit

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Type is the audit event type (closed vocabulary).
type Type string

// Outcome is the audit event outcome.
type Outcome string

// Reason is the closed vocabulary of reasons. Values are never peer-supplied.
type Reason string

// Event types.
const (
	TypeAuthnRefused         Type = "authn_refused"
	TypeAuthzRefused         Type = "authz_refused"
	TypeAuthzAllowed         Type = "authz_allowed"
	TypeIdentityIssued       Type = "identity_issued"
	TypeIdentityRenewed      Type = "identity_renewed"
	TypeIdentityRenewFailed  Type = "identity_renewal_failed"
	TypeIdentityExpired      Type = "identity_expired"
	TypeTrustBundleUpdated   Type = "trust_bundle_updated"
	TypePolicyLoaded         Type = "policy_loaded"
	TypePolicyLoadFailed     Type = "policy_load_failed"
	TypeInsecureModeEnabled  Type = "insecure_mode_enabled"
	TypeLimitExceeded        Type = "limit_exceeded"
	TypeChannelRefusedDowngr Type = "channel_refused_downgrade"
)

// Outcomes.
const (
	OutcomeRefused Outcome = "refused"
	OutcomeOK      Outcome = "ok"
	OutcomeFailed  Outcome = "failed"
)

// Reasons.
const (
	ReasonNoIdentity              Reason = "no_identity"
	ReasonUntrusted               Reason = "untrusted"
	ReasonIdentityExpired         Reason = "identity_expired"
	ReasonIdentityNotYetValid     Reason = "identity_not_yet_valid"
	ReasonIdentityRevoked         Reason = "identity_revoked"
	ReasonNameMismatch            Reason = "name_mismatch"
	ReasonDowngradeRefused        Reason = "downgrade_refused"
	ReasonExplicitAllow           Reason = "explicit_allow"
	ReasonExplicitDeny            Reason = "explicit_deny"
	ReasonNoMatchingRule          Reason = "no_matching_rule"
	ReasonNoPolicy                Reason = "no_policy"
	ReasonLimitExceeded           Reason = "limit_exceeded"
	ReasonRenewed                 Reason = "renewed"
	ReasonIssued                  Reason = "issued"
	ReasonProviderUnavailable     Reason = "provider_unavailable"
	ReasonBundleEmpty             Reason = "bundle_empty"
	ReasonPolicyInvalid           Reason = "policy_invalid"
	ReasonPolicySourceUnavailable Reason = "policy_source_unavailable"
	ReasonLocalDev                Reason = "local_dev"
	ReasonAllowAll                Reason = "allow_all"
	ReasonBundleUpdated           Reason = "bundle_updated"
	ReasonLoaded                  Reason = "loaded"
)

var (
	allTypes = []Type{
		TypeAuthnRefused, TypeAuthzRefused, TypeAuthzAllowed, TypeIdentityIssued, TypeIdentityRenewed,
		TypeIdentityRenewFailed, TypeIdentityExpired, TypeTrustBundleUpdated, TypePolicyLoaded,
		TypePolicyLoadFailed, TypeInsecureModeEnabled, TypeLimitExceeded, TypeChannelRefusedDowngr,
	}
	allReasons = []Reason{
		ReasonNoIdentity, ReasonUntrusted, ReasonIdentityExpired, ReasonIdentityNotYetValid,
		ReasonIdentityRevoked, ReasonNameMismatch, ReasonDowngradeRefused, ReasonExplicitAllow,
		ReasonExplicitDeny, ReasonNoMatchingRule, ReasonNoPolicy, ReasonLimitExceeded, ReasonRenewed,
		ReasonIssued, ReasonProviderUnavailable, ReasonBundleEmpty, ReasonPolicyInvalid,
		ReasonPolicySourceUnavailable, ReasonLocalDev, ReasonAllowAll, ReasonBundleUpdated, ReasonLoaded,
	}
	typeSet    = toSet(allTypes)
	reasonSet  = toSet(allReasons)
	outcomeSet = map[Outcome]struct{}{OutcomeRefused: {}, OutcomeOK: {}, OutcomeFailed: {}}

	correlationRE = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)
	traceIDRE     = regexp.MustCompile(`^[0-9a-f]{32}$`)
)

func toSet[T comparable](xs []T) map[T]struct{} {
	m := make(map[T]struct{}, len(xs))
	for _, x := range xs {
		m[x] = struct{}{}
	}
	return m
}

// AllTypes returns every defined event type.
func AllTypes() []Type { return append([]Type(nil), allTypes...) }

// AllReasons returns every defined reason.
func AllReasons() []Reason { return append([]Reason(nil), allReasons...) }

// Event is an immutable audit record. Field names match contracts/audit-event.schema.json.
type Event struct {
	Time           time.Time         `json:"time"`
	Type           Type              `json:"type"`
	Outcome        Outcome           `json:"outcome"`
	Reason         Reason            `json:"reason"`
	LocalID        string            `json:"local_id"`
	ClaimedPeerID  string            `json:"claimed_peer_id,omitempty"`
	VerifiedPeerID string            `json:"verified_peer_id,omitempty"`
	Operation      string            `json:"operation,omitempty"`
	CorrelationID  string            `json:"correlation_id"`
	TraceID        string            `json:"trace_id,omitempty"`
	RuleID         string            `json:"rule_id,omitempty"`
	PolicyVersion  string            `json:"policy_version,omitempty"`
	RemoteAddr     string            `json:"remote_addr,omitempty"`
	Attrs          map[string]string `json:"attrs,omitempty"`
}

const timeLayout = "2006-01-02T15:04:05.000000000Z07:00"

type alias Event

// MarshalJSON encodes Time as RFC 3339 with nanoseconds in UTC.
func (e Event) MarshalJSON() ([]byte, error) {
	a := alias(e)
	a.Time = time.Time{}
	return json.Marshal(struct {
		Time string `json:"time"`
		alias
	}{e.Time.UTC().Format(timeLayout), a})
}

// UnmarshalJSON accepts any RFC 3339 timestamp.
func (e *Event) UnmarshalJSON(b []byte) error {
	var raw struct {
		Time string `json:"time"`
		alias
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	*e = Event(raw.alias)
	if raw.Time != "" {
		t, err := time.Parse(time.RFC3339Nano, raw.Time)
		if err != nil {
			return fmt.Errorf("audit: time: %w", err)
		}
		e.Time = t
	}
	return nil
}

// Validate enforces the schema and the closed vocabularies.
func (e Event) Validate() error {
	if e.Time.IsZero() {
		return errors.New("audit: time is required")
	}
	if _, ok := typeSet[e.Type]; !ok {
		return fmt.Errorf("audit: unknown type %q", e.Type)
	}
	if _, ok := outcomeSet[e.Outcome]; !ok {
		return fmt.Errorf("audit: unknown outcome %q", e.Outcome)
	}
	if _, ok := reasonSet[e.Reason]; !ok {
		return fmt.Errorf("audit: unknown reason %q", e.Reason)
	}
	if !strings.HasPrefix(e.LocalID, "spiffe://") {
		return errors.New("audit: local_id must be a SPIFFE ID")
	}
	if !correlationRE.MatchString(e.CorrelationID) {
		return errors.New("audit: correlation_id must match ^[A-Za-z0-9._-]{1,128}$")
	}
	if e.TraceID != "" && !traceIDRE.MatchString(e.TraceID) {
		return errors.New("audit: trace_id must be 32 hex characters")
	}
	if len(e.Operation) > 256 {
		return errors.New("audit: operation longer than 256")
	}
	return nil
}
