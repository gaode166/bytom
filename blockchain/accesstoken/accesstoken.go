// Package accesstoken provides storage and validation of Chain Core
// credentials.
package accesstoken

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	dbm "github.com/tendermint/tmlibs/db"

	"github.com/bytom/crypto/sha3pool"
	"github.com/bytom/errors"
)

const (
	tokenSize          = 32
	defGenericPageSize = 100
)

var (
	// ErrBadID is returned when Create is called on an invalid id string.
	ErrBadID = errors.New("invalid id")
	// ErrDuplicateID is returned when Create is called on an existing ID.
	ErrDuplicateID = errors.New("duplicate access token ID")
	// ErrBadType is returned when Create is called with a bad type.
	ErrBadType = errors.New("type must be client or network")
	// ErrNoMatchID is returned when Delete is called on nonexisting ID.
	ErrNoMatchID = errors.New("nonexisting access token ID")

	// validIDRegexp checks that all characters are alphumeric, _ or -.
	// It also must have a length of at least 1.
	validIDRegexp = regexp.MustCompile(`^[\w-]+$`)
)

// Token describe the access token.
type Token struct {
	ID      string    `json:"id"`
	Token   string    `json:"token,omitempty"`
	Type    string    `json:"type,omitempty"`
	Created time.Time `json:"created_at"`
}

// CredentialStore store user access credential.
type CredentialStore struct {
	DB dbm.DB
}

// NewStore creates and returns a new Store object.
func NewStore(db dbm.DB) *CredentialStore {
	return &CredentialStore{
		DB: db,
	}
}

// Create generates a new access token with the given ID.
func (cs *CredentialStore) Create(ctx context.Context, id, typ string) (*string, error) {
	if !validIDRegexp.MatchString(id) {
		return nil, errors.WithDetailf(ErrBadID, "invalid id %q", id)
	}
	k, err := json.Marshal(id)
	if v := cs.DB.Get(k); v != nil {
		return nil, errors.WithDetailf(ErrDuplicateID, "id %q already in use", id)
	}
	var secret [tokenSize]byte
	v, err := rand.Read(secret[:])
	if err != nil || v != tokenSize {
		return nil, err
	}

	var hashedSecret [tokenSize]byte
	sha3pool.Sum256(hashedSecret[:], secret[:])
	created := time.Now()

	token := &Token{
		ID:      id,
		Token:   fmt.Sprintf("%s:%x", id, hashedSecret),
		Type:    typ,
		Created: created,
	}

	key, err := json.Marshal(id)
	if err != nil {
		return nil, err
	}
	value, err := json.Marshal(token)
	if err != nil {
		return nil, err
	}
	cs.DB.Set(key, value)
	hexsec := fmt.Sprintf("%s:%x", id, secret)
	return &hexsec, nil
}

// Check returns whether or not an id-secret pair is a valid access token.
func (cs *CredentialStore) Check(ctx context.Context, id string, secret []byte) (bool, error) {
	if !validIDRegexp.MatchString(id) {
		return false, errors.WithDetailf(ErrBadID, "invalid id %q", id)
	}

	var toHash [tokenSize]byte
	var hashed [tokenSize]byte
	copy(toHash[:], secret)
	sha3pool.Sum256(hashed[:], toHash[:])
	inToken := fmt.Sprintf("%s:%x", id, hashed[:])

	var value []byte
	token := &Token{}
	k, err := json.Marshal(id)
	if err != nil {
		return false, err
	}

	if value = cs.DB.Get(k); value == nil {
		return false, errors.WithDetailf(ErrNoMatchID, "check id %q nonexisting", id)
	}
	if err := json.Unmarshal(value, token); err != nil {
		return false, err
	}

	if strings.Compare(token.Token, inToken) == 0 {
		return true, nil
	}

	return false, nil
}

// List lists all access tokens.
func (cs *CredentialStore) List(after string, limit, defaultLimit int) ([]string, string, bool, error) {
	var (
		zafter int
		err    error
		last   bool
	)

	if after != "" {
		zafter, err = strconv.Atoi(after)
		if err != nil {
			return nil, "", false, errors.WithDetailf(errors.New("Invalid after"), "value: %q", zafter)
		}
	}

	tokens := make([]string, 0)
	iter := cs.DB.Iterator()
	defer iter.Release()

	for iter.Next() {
		tokens = append(tokens, string(iter.Value()))
	}

	start, end := 0, len(tokens)

	if len(tokens) == 0 {
		return nil, "", true, errors.New("No access token")
	} else if len(tokens) > zafter {
		start = zafter
	} else {
		return nil, "", false, errors.WithDetailf(errors.New("Invalid after"), "value: %q", zafter)
	}

	if len(tokens) > zafter+limit {
		end = zafter + limit
	}
	if len(tokens) == end || len(tokens) < defaultLimit {
		last = true
	}

	return tokens[start:end], strconv.Itoa(end), last, nil
}

// Delete deletes an access token by id.
func (cs *CredentialStore) Delete(ctx context.Context, id string) error {
	if !validIDRegexp.MatchString(id) {
		return errors.WithDetailf(ErrBadID, "invalid id %q", id)
	}
	k, err := json.Marshal(id)
	if err != nil {
		return err
	}
	cs.DB.Delete(k)

	return nil
}
