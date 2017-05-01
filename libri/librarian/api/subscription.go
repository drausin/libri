package api

import "errors"

// ErrEmptySubscriptionFilters indicates when a *Subscription has an empty author or reader public
// key filter.
var ErrEmptySubscriptionFilters = errors.New("subscription has empty filters")

// ErrUnexpectedNilValue indicates when a value is unexpectedly nil.
var ErrUnexpectedNilValue = errors.New("unexpected nil value")

// ValidateSubscription validates that a subscription is not missing any required fields. It returns
// nil if the subscription is valid.
func ValidateSubscription(s *Subscription) error {
	if s == nil {
		return ErrUnexpectedNilValue
	}
	if s.AuthorPublicKeys == nil || s.AuthorPublicKeys.Encoded == nil {
		return ErrEmptySubscriptionFilters
	}
	if s.ReaderPublicKeys == nil || s.ReaderPublicKeys.Encoded == nil {
		return ErrEmptySubscriptionFilters
	}
	return nil
}

// ValidatePublication validates that a publication has all fields of the correct length.
func ValidatePublication(p *Publication) error {
	if p == nil {
		return ErrUnexpectedNilValue
	}
	if err := ValidateBytes(p.EntryKey, DocumentKeyLength, "EntryKey"); err != nil {
		return err
	}
	if err := ValidateBytes(p.EnvelopeKey, DocumentKeyLength, "EnvelopeKey"); err != nil {
		return err
	}
	if err := ValidatePublicKey(p.AuthorPublicKey); err != nil {
		return err
	}
	if err := ValidatePublicKey(p.ReaderPublicKey); err != nil {
		return err
	}
	return nil
}