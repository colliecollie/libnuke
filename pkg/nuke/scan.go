package nuke

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"

	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/semaphore"

	"github.com/rebuy-de/aws-nuke/pkg/awsutil"

	"github.com/ekristen/cloud-nuke-sdk/pkg/queue"
	"github.com/ekristen/cloud-nuke-sdk/pkg/resource"
	"github.com/ekristen/cloud-nuke-sdk/pkg/utils"
)

const ScannerParallelQueries = 16

type Scanner struct {
	items     chan *queue.Item
	semaphore *semaphore.Weighted
}

func NewScanner() *Scanner {
	return &Scanner{
		items:     make(chan *queue.Item, 100),
		semaphore: semaphore.NewWeighted(ScannerParallelQueries),
	}
}

type IScanner interface {
	run(resourceTypes []string)
	list(resourceType string)
}

func (s *Scanner) run(resourceTypes []string, opts interface{}) {
	ctx := context.Background()

	for _, resourceType := range resourceTypes {
		s.semaphore.Acquire(ctx, 1)
		go s.list(resourceType, opts)
	}

	// Wait for all routines to finish.
	s.semaphore.Acquire(ctx, ScannerParallelQueries)

	close(s.items)
}

func (s *Scanner) list(resourceType string, opts interface{}) {
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("%v\n\n%s", r.(error), string(debug.Stack()))
			dump := utils.Indent(fmt.Sprintf("%v", err), "    ")
			log.Errorf("Listing %s failed:\n%s", resourceType, dump)
		}
	}()
	defer s.semaphore.Release(1)

	lister := resource.GetLister(resourceType)
	var rs []resource.Resource
	lister.SetOptions(opts)
	rs, err := lister.List()
	if err != nil {
		var errSkipRequest awsutil.ErrSkipRequest
		ok := errors.As(err, &errSkipRequest)
		if ok {
			log.Debugf("skipping request: %v", err)
			return
		}

		var errUnknownEndpoint awsutil.ErrUnknownEndpoint
		ok = errors.As(err, &errUnknownEndpoint)
		if ok {
			log.Warnf("skipping request: %v", err)
			return
		}

		dump := utils.Indent(fmt.Sprintf("%v", err), "    ")
		log.Errorf("Listing %s failed:\n%s", resourceType, dump)
		return
	}

	for _, r := range rs {
		i := &queue.Item{
			Resource: r,
			State:    queue.ItemStateNew,
			Type:     resourceType,
		}
		s.items <- i
	}
}
