package biz

import (
	"context"

	"entgo.io/contrib/entgql"
	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/sqljson"

	"github.com/looplj/axonhub/internal/ent"
)

// QueryChannelsInput represents the input for querying channels with additional filters.
type QueryChannelsInput struct {
	After   *entgql.Cursor[int]
	First   *int
	Before  *entgql.Cursor[int]
	Last    *int
	OrderBy *ent.ChannelOrder
	Where   *ent.ChannelWhereInput
	HasTag  *string
	Model   *string
}

// QueryChannels queries channels with the specified input parameters, including model filtering.
func (svc *ChannelService) QueryChannels(ctx context.Context, input QueryChannelsInput) (*ent.ChannelConnection, error) {
	// Build the base query
	var (
		query = svc.entFromContext(ctx).Channel.Query()
		err   error
	)

	// Apply standard filters
	if input.Where != nil {
		query, err = input.Where.Filter(query)
		if err != nil {
			return nil, err
		}
	}

	if input.HasTag != nil && *input.HasTag != "" {
		query = query.Where(func(s *sql.Selector) {
			s.Where(sqljson.ValueContains("tags", *input.HasTag))
		})
	}

	// If the model is not specified, return the query result directly.
	if input.Model == nil || *input.Model == "" {
		return query.Paginate(ctx, input.After, input.First, input.Before, input.Last,
			ent.WithChannelOrder(input.OrderBy),
		)
	}

	// When model filtering is required, we fetch all results and filter in-memory, bypassing database pagination.
	return svc.queryChannelsWithModelFilter(ctx, query, input)
}

// queryChannelsWithModelFilter performs model filtering without pagination.
// When model filtering is required, return all matching channels without pagination.
func (svc *ChannelService) queryChannelsWithModelFilter(
	ctx context.Context,
	query *ent.ChannelQuery,
	input QueryChannelsInput,
) (*ent.ChannelConnection, error) {
	// Fetch all channels from the database
	if input.OrderBy != nil {
		query = query.Order(input.OrderBy.ToOrderOption())
	}

	channels, err := query.All(ctx)
	if err != nil {
		return nil, err
	}

	// Filter channels by model support
	var filteredChannels []*ent.Channel
	for _, channel := range channels {
		channelObj := Channel{Channel: channel}
		if channelObj.IsModelSupported(*input.Model) {
			filteredChannels = append(filteredChannels, channel)
		}
	}

	// Build connection without pagination (ignore all pagination params for model filtering)
	return svc.buildConnectionInMemory(filteredChannels, input.OrderBy), nil
}

// buildConnectionInMemory builds a relay-style connection from filtered channels.
func (svc *ChannelService) buildConnectionInMemory(
	channels []*ent.Channel,
	order *ent.ChannelOrder,
) *ent.ChannelConnection {
	conn := &ent.ChannelConnection{
		Edges:    []*ent.ChannelEdge{},
		PageInfo: ent.PageInfo{},
	}

	// Handle empty result
	if len(channels) == 0 {
		conn.TotalCount = 0
		return conn
	}

	// Return all channels without pagination
	conn.Edges = make([]*ent.ChannelEdge, len(channels))
	for i, ch := range channels {
		conn.Edges[i] = ch.ToEdge(order)
	}

	conn.PageInfo.HasNextPage = false
	conn.PageInfo.HasPreviousPage = false

	conn.TotalCount = len(channels)
	if len(conn.Edges) > 0 {
		conn.PageInfo.StartCursor = &conn.Edges[0].Cursor
		conn.PageInfo.EndCursor = &conn.Edges[len(conn.Edges)-1].Cursor
	}

	return conn
}
