package server

import (
	"context"
	"net/url"
	"strconv"

	soundcloudapi "github.com/zackradisic/soundcloud-api"
)

// getLikesBulk is for getting likes where track count is > 200. arr should be a pointer to
// an array of Likes whose length is equal to options.Limit.
//
// This is commented out because
/*
func (s *Server) getLikesBulk(ctx context.Context, arr *[]soundcloudapi.Like, options soundcloudapi.GetLikesOptions) error {
	log.Panic("DON'T USE THIS")
	g, ctx := errgroup.WithContext(ctx)

	var m *sync.Mutex = &sync.Mutex{}
	workers := int(math.Ceil(float64(options.Limit) / float64(200)))
	index := 0 // keep track of the current index for arr
	for i := 0; i < workers; i++ {
		x := i
		id := options.ID
		total := options.Limit
		g.Go(func() error {
			var limit = 200
			var offset = limit * x
			// The last worker might fetch less than 200 likes
			if x == workers-1 {
				limit = total % 200
			}
			query, err := s.scdl.GetLikes(soundcloudapi.GetLikesOptions{
				ID:     id,
				Limit:  limit,
				Offset: offset,
				Type:   "track",
			})
			if err == nil {
				m.Lock()
				defer m.Unlock()
				likes, err := query.GetLikes()
				fmt.Printf("Offset: %d Limit: %d LENGTH: %d\n", offset, limit, len(likes))
				if err == nil {
					for _, like := range likes {
						(*arr)[index] = like
						index++
					}
				}
			}
			return err
		})
	}

	return g.Wait()
}
*/

func (s *Server) getLikesBulk(ctx context.Context, arr *[]soundcloudapi.Like, options soundcloudapi.GetLikesOptions) error {
	i := 0
	for {
		likes, err := s.scdl.GetLikes(options)
		if err != nil {
			return err
		}

		l, err := likes.GetLikes()
		if err != nil {
			return err
		}

		for _, like := range l {
			(*arr)[i] = like
			i++
		}

		if likes.NextHref == "" {
			return nil
		}

		u, err := url.Parse(likes.NextHref)
		if err != nil {
			return err
		}
		options.Offset, err = strconv.Atoi(u.Query().Get("offset"))
		if err != nil {
			return err
		}
		if i >= options.Limit {
			return err
		}
	}
}
