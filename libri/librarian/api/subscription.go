package api

import "errors"

var (
	// ErrEmptySubscriptionFilters indicates when a *Subscription has an empty author or reader
	// public key filter.
	ErrEmptySubscriptionFilters = errors.New("subscription has empty filters")

	// ErrMissingSubscription indicates when a Subscription is unexpectedly nil.
	ErrMissingSubscription = errors.New("missing Subscription")

	// ErrMissingPublication indicates when a Publication is unexpectedly nil.
	ErrMissingPublication = errors.New("missing Publication")
)

// ValidateSubscription validates that a subscription is not missing any required fields. It returns
// nil if the subscription is valid.
func ValidateSubscription(s *Subscription) error {
	if s == nil {
		return ErrMissingSubscription
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
		return ErrMissingPublication
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

// GetPublication returns a *Publication object if one can be made from the given document key and
// value. If not, it returns nil.
func GetPublication(key []byte, value *Document) *Publication {
	switch x := value.Contents.(type) {
	case *Document_Envelope:
		return &Publication{
			EntryKey:        x.Envelope.EntryKey,
			EnvelopeKey:     key,
			AuthorPublicKey: x.Envelope.AuthorPublicKey,
			ReaderPublicKey: x.Envelope.ReaderPublicKey,
		}
	}
	return nil
}
