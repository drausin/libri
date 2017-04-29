package api

import "errors"

var ErrEmptySubscriptionFilters = errors.New("subscription has empty filters")

func ValidateSubscription(s *Subscription) error {
	if s.AuthorPublicKeys == nil || s.AuthorPublicKeys.Encoded == nil {
		return ErrEmptySubscriptionFilters
	}
	if s.ReaderPublicKeys == nil || s.ReaderPublicKeys.Encoded == nil {
		return ErrEmptySubscriptionFilters
	}
	return nil
}

func ValidatePublication(p *Publication) error {
	if p == nil {
		return errors.New("Publication may not be nil")
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