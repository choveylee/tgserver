package tgserver

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/choveylee/tlog"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const payloadTruncated = "...(truncated)"

var sensitiveNameSeparators = strings.NewReplacer("_", "", "-", "", ".", "", " ", "")

// marshalProtoMessage makes a bounded diagnostic copy. Any values and byte
// fields are opaque and are excluded from the logged copy.
func marshalProtoMessage(ctx context.Context, payload any, maxBytes int) (serialized string) {
	message, ok := payload.(proto.Message)
	if !ok || isNilValue(message) {
		return ""
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			tlog.W(ctx).Msgf("Encode gRPC payload for logging error (panic type=%T)", recovered)
			serialized = ""
		}
	}()

	if proto.Size(message) > maxBytes {
		return payloadTruncated[:min(maxBytes, len(payloadTruncated))]
	}

	redacted := proto.Clone(message)
	redactProtoMessage(redacted.ProtoReflect())

	data, err := protojson.Marshal(redacted)
	if err != nil {
		tlog.W(ctx).Msgf("Encode gRPC payload for logging error (error type=%T)", err)

		return ""
	}

	return truncatePayloadForLog(data, maxBytes)
}

// truncatePayloadForLog keeps the marker within the byte budget and preserves UTF-8.
func truncatePayloadForLog(data []byte, maxBytes int) string {
	if len(data) <= maxBytes {
		return string(data)
	}

	if maxBytes <= len(payloadTruncated) {
		return payloadTruncated[:maxBytes]
	}

	end := maxBytes - len(payloadTruncated)
	for end > 0 && !utf8.RuneStart(data[end]) {
		end--
	}

	return string(data[:end]) + payloadTruncated
}

// redactProtoMessage walks mutable copies only. Any envelopes are emptied
// because their contents are invisible to the outer message descriptors.
func redactProtoMessage(message protoreflect.Message) {
	if !message.IsValid() {
		return
	}

	isAny := message.Descriptor().FullName() == "google.protobuf.Any"
	message.Range(func(field protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		switch {
		case isAny || sensitivePayloadName(string(field.Name())) || sensitivePayloadName(field.JSONName()) || field.Kind() == protoreflect.BytesKind:
			message.Clear(field)
		case field.IsMap():
			valueMap := value.Map()
			valueKind := field.MapValue().Kind()
			valueMap.Range(func(key protoreflect.MapKey, entry protoreflect.Value) bool {
				if valueKind == protoreflect.BytesKind || (field.MapKey().Kind() == protoreflect.StringKind && sensitivePayloadName(key.String())) {
					valueMap.Clear(key)
				} else if valueKind == protoreflect.MessageKind {
					redactProtoMessage(entry.Message())
				}

				return true
			})
		case field.Kind() == protoreflect.MessageKind || field.Kind() == protoreflect.GroupKind:
			if field.IsList() {
				values := value.List()
				for index := 0; index < values.Len(); index++ {
					redactProtoMessage(values.Get(index).Message())
				}
			} else {
				redactProtoMessage(value.Message())
			}
		}

		return true
	})
}

func sensitivePayloadName(name string) bool {
	normalized := sensitiveNameSeparators.Replace(strings.ToLower(name))
	for _, keyword := range [...]string{"password", "passwd", "token", "secret", "authorization", "cookie", "apikey", "privatekey", "credential"} {
		if strings.Contains(normalized, keyword) {
			return true
		}
	}

	return false
}
