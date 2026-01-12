package agens

import (
	"errors"

	"github.com/firebase/genkit/go/ai"
)

const (
	// ChannelIDKey is the key used in message metadata to store the
	// unique identifier for a channel.
	ChannelIDKey = "channel_id"

	// SourceKey is the key used in message metadata to store the source of the message
	SourceKey = "source"

	// StoredIDKey is the key used in the message metadata to store the unique identifier
	// of the message with which it was stored in a database or external system.
	StoredIDKey = "stored_id"

	// UserIDKey is the key used in message metadata to store the ID of
	// the user who sent the message.
	UserIDKey = "user_id"
)

var (
	// ErrMetadataNotFound is returned if the message metadata map is nil.
	ErrMetadataNotFound = errors.New("metadata not found")

	// ErrChannelIDNotInMetadata is returned if the channel ID is not present
	// in the message metadata.
	ErrChannelIDNotInMetadata = errors.New("channel ID not found in metadata")

	// ErrSourceNotInMetadata is returned if the source is not present
	// in the message metadata.
	ErrSourceNotInMetadata = errors.New("source not found in metadata")

	// ErrUserIDNotInMetadata is returned if the user ID is not present
	// in the message metadata.
	ErrUserIDNotInMetadata = errors.New("user ID not found in metadata")

	// ErrChannelIDNotAString is returned if the channel ID in metadata
	// is not a string.
	ErrChannelIDNotAString = errors.New("channel ID is not a string type")

	// ErrSourceNotAString is returned if the source in metadata is not a string.
	ErrSourceNotAString = errors.New("source is not a string type")

	// ErrStoredIDNotAString is returned if the stored ID in metadata is not a string.
	ErrStoredIDNotAString = errors.New("stored ID is not a string type")

	// ErrUserIDNotAString is returned if the user ID in metadata is not a string.
	ErrUserIDNotAString = errors.New("user ID is not a string type")
)

func getMetadata(msg *ai.Message, key string) (value any, ok bool, err error) {
	if (msg == nil) || (msg.Metadata == nil) {
		return nil, false, ErrMetadataNotFound
	}

	v, ok := msg.Metadata[key]
	return v, ok, nil
}

func setMetadata(msg *ai.Message, key string, value any) *ai.Message {
	if msg.Metadata == nil {
		msg.Metadata = make(map[string]any)
	}
	msg.Metadata[key] = value
	return msg
}

// GetChannelID retrieves the channel ID from a message's metadata.
// It returns the ID as a string and an error if the ID is missing or invalid.
func GetChannelID(msg *ai.Message) (string, error) {
	v, ok, err := getMetadata(msg, ChannelIDKey)
	if err != nil {
		return "", err
	} else if !ok {
		return "", ErrChannelIDNotInMetadata
	}

	if id, ok := v.(string); ok {
		return id, nil
	}
	return "", ErrChannelIDNotAString
}

// GetSource retrieves the message source from a message's metadata.
// It returns the source as a string and an error if the key is missing or invalid.
func GetSource(msg *ai.Message) (string, error) {
	v, ok, err := getMetadata(msg, SourceKey)
	if err != nil {
		return "", err
	} else if !ok {
		return "", ErrSourceNotInMetadata
	}

	if source, ok := v.(string); ok {
		return source, nil
	}
	return "", ErrSourceNotAString
}

// GetStoredID retrieves the unique stored identifier from a message's metadata.
func GetStoredID(msg *ai.Message) (string, error) {
	v, ok, _ := getMetadata(msg, StoredIDKey)
	if !ok {
		return "", nil
	}

	if id, ok := v.(string); ok {
		return id, nil
	}
	return "", ErrStoredIDNotAString
}

// GetUserID retrieves the user ID from a message's metadata.
// It returns the user ID as a string and an error if the key is missing or invalid.
func GetUserID(msg *ai.Message) (string, error) {
	v, ok, err := getMetadata(msg, UserIDKey)
	if err != nil {
		return "", err
	} else if !ok {
		return "", ErrUserIDNotInMetadata
	}

	if id, ok := v.(string); ok {
		return id, nil
	}
	return "", ErrUserIDNotAString
}

// SetChannelID sets the channel ID in a message's metadata.
func SetChannelID(msg *ai.Message, id string) *ai.Message {
	return setMetadata(msg, ChannelIDKey, id)
}

// SetSource sets the message source in a message's metadata.
func SetSource(msg *ai.Message, source string) *ai.Message {
	return setMetadata(msg, SourceKey, source)
}

// SetStoredID sets the unique stored identifier in a message's metadata.
func SetStoredID(msg *ai.Message, id string) *ai.Message {
	return setMetadata(msg, StoredIDKey, id)
}

// SetUserID sets the user ID in a message's metadata.
func SetUserID(msg *ai.Message, id string) *ai.Message {
	return setMetadata(msg, UserIDKey, id)
}
