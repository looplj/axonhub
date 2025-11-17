package ent

import (
	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/sqljson"
)

func (r *Role) IsSystemRole() bool {
	return r.ProjectID == nil || *r.ProjectID == 0
}

// WithChannelTagFilter add tag filter to channel pagination.
func WithChannelTagFilter(hasTag *string) ChannelPaginateOption {
	return func(pager *channelPager) error {
		if hasTag == nil {
			return nil
		}

		if pager.filter == nil {
			pager.filter = func(q *ChannelQuery) (*ChannelQuery, error) {
				return q.Where(func(s *sql.Selector) {
					s.Where(sqljson.ValueContains("tags", *hasTag))
				}), nil
			}
		} else {
			filter := pager.filter
			pager.filter = func(q *ChannelQuery) (*ChannelQuery, error) {
				q, err := filter(q)
				if err != nil {
					return nil, err
				}

				return q.Where(func(s *sql.Selector) {
					s.Where(sqljson.ValueContains("tags", *hasTag))
				}), nil
			}
		}

		return nil
	}
}
