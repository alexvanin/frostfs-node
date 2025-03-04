package getsvc

import (
	"context"

	"github.com/TrueCloudLab/frostfs-node/pkg/util"
	"github.com/TrueCloudLab/frostfs-sdk-go/object"
	"go.uber.org/zap"
)

// Get serves a request to get an object by address, and returns Streamer instance.
func (s *Service) Get(ctx context.Context, prm Prm) error {
	return s.get(ctx, prm.commonPrm).err
}

// GetRange serves a request to get an object by address, and returns Streamer instance.
func (s *Service) GetRange(ctx context.Context, prm RangePrm) error {
	return s.getRange(ctx, prm)
}

func (s *Service) getRange(ctx context.Context, prm RangePrm, opts ...execOption) error {
	return s.get(ctx, prm.commonPrm, append(opts, withPayloadRange(prm.rng))...).err
}

func (s *Service) GetRangeHash(ctx context.Context, prm RangeHashPrm) (*RangeHashRes, error) {
	hashes := make([][]byte, 0, len(prm.rngs))

	for _, rng := range prm.rngs {
		h := prm.hashGen()

		// For big ranges we could fetch range-hashes from different nodes and concatenate them locally.
		// However,
		// 1. Potential gains are insignificant when operating in the Internet given typical latencies and losses.
		// 2. Parallel solution is more complex in terms of code.
		// 3. TZ-hash is likely to be disabled in private installations.
		rngPrm := RangePrm{
			commonPrm: prm.commonPrm,
		}

		rngPrm.SetRange(&rng)
		rngPrm.SetChunkWriter(&hasherWrapper{
			hash: util.NewSaltingWriter(h, prm.salt),
		})

		if err := s.getRange(ctx, rngPrm); err != nil {
			return nil, err
		}

		hashes = append(hashes, h.Sum(nil))
	}

	return &RangeHashRes{
		hashes: hashes,
	}, nil
}

// Head reads object header from container.
//
// Returns ErrNotFound if the header was not received for the call.
// Returns SplitInfoError if object is virtual and raw flag is set.
func (s *Service) Head(ctx context.Context, prm HeadPrm) error {
	return s.get(ctx, prm.commonPrm, headOnly()).err
}

func (s *Service) get(ctx context.Context, prm commonPrm, opts ...execOption) statusError {
	exec := &execCtx{
		svc: s,
		ctx: ctx,
		prm: RangePrm{
			commonPrm: prm,
		},
		infoSplit: object.NewSplitInfo(),
	}

	for i := range opts {
		opts[i](exec)
	}

	exec.setLogger(s.log)

	exec.execute()

	return exec.statusError
}

func (exec *execCtx) execute() {
	exec.log.Debug("serving request...")

	// perform local operation
	exec.executeLocal()

	exec.analyzeStatus(true)
}

func (exec *execCtx) analyzeStatus(execCnr bool) {
	// analyze local result
	switch exec.status {
	case statusOK:
		exec.log.Debug("operation finished successfully")
	case statusINHUMED:
		exec.log.Debug("requested object was marked as removed")
	case statusVIRTUAL:
		exec.log.Debug("requested object is virtual")
		exec.assemble()
	case statusOutOfRange:
		exec.log.Debug("requested range is out of object bounds")
	default:
		exec.log.Debug("operation finished with error",
			zap.String("error", exec.err.Error()),
		)

		if execCnr {
			exec.executeOnContainer()
			exec.analyzeStatus(false)
		}
	}
}
