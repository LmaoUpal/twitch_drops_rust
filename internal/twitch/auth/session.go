package auth

import (
	"context"
	"errors"
)

// Session ties a Client to a Store for one account.
type Session struct {
	Client  *Client
	Store   Store
	Account string
}

// Restore loads the saved token and makes sure it still works, refreshing it if needed.
// It returns ErrNoToken when the user must log in again. Network errors are returned
// as-is and leave the saved token untouched.
func (s *Session) Restore(ctx context.Context) (*Token, error) {
	tok, err := s.Store.Load(s.Account)
	if err != nil {
		return nil, err
	}
	v, err := s.Client.Validate(ctx, tok.AccessToken)
	if err == nil {
		return s.update(tok, v, false)
	}
	if !errors.Is(err, ErrInvalidToken) {
		return nil, err
	}
	if tok.RefreshToken == "" {
		return nil, s.forget()
	}
	fresh, err := s.Client.Refresh(ctx, tok.RefreshToken)
	if errors.Is(err, ErrInvalidToken) {
		return nil, s.forget()
	}
	if err != nil {
		return nil, err
	}
	return s.Complete(ctx, fresh)
}

// Complete validates a newly obtained token, fills in the user identity and saves it.
func (s *Session) Complete(ctx context.Context, tok *Token) (*Token, error) {
	v, err := s.Client.Validate(ctx, tok.AccessToken)
	if err != nil {
		return nil, err
	}
	return s.update(tok, v, true)
}

// Logout removes the saved token.
func (s *Session) Logout() error {
	return s.Store.Delete(s.Account)
}

// update fills identity from v and saves when forced or when the identity changed.
func (s *Session) update(tok *Token, v *Validation, force bool) (*Token, error) {
	changed := tok.UserID != v.UserID || tok.Login != v.Login
	tok.UserID, tok.Login = v.UserID, v.Login
	if force || changed {
		if err := s.Store.Save(s.Account, tok); err != nil {
			return nil, err
		}
	}
	return tok, nil
}

func (s *Session) forget() error {
	if err := s.Store.Delete(s.Account); err != nil {
		return err
	}
	return ErrNoToken
}
