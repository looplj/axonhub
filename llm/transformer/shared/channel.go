package shared

import "context"

type channelIDContextKey struct{}

func WithChannelID(ctx context.Context, channelID int) context.Context {
	return context.WithValue(ctx, channelIDContextKey{}, channelID)
}

func GetChannelID(ctx context.Context) (int, bool) {
	channelID, ok := ctx.Value(channelIDContextKey{}).(int)
	return channelID, ok
}
