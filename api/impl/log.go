package impl

import (
	"bufio"
	"context"
	"encoding/json"
	"io"

	manet "gx/ipfs/QmV6FjemM1K8oXjrvuq3wuVWWoU2TLDPmNnKrxHzY3v6Ai/go-multiaddr-net"
	ma "gx/ipfs/QmYmsdtJ3HsodkePE3eU3TsCaP2YvPZJ4LoXnNkDE5Tpt7/go-multiaddr"
	multidns "gx/ipfs/QmfXU2MhWoegxHoeMd3A2ytL2P6CY4FfqGWc23LTNWBwZt/go-multiaddr-dns"

	errors "gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	logging "gx/ipfs/QmZChCsSt8DctjceaL56Eibc29CVQq4dGKRXC5JRZ6Ppae/go-log"
	writer "gx/ipfs/QmZChCsSt8DctjceaL56Eibc29CVQq4dGKRXC5JRZ6Ppae/go-log/writer"
)

var log = logging.Logger("api/impl")

type nodeLog struct {
	api *nodeAPI
}

func newNodeLog(api *nodeAPI) *nodeLog {
	return &nodeLog{api: api}
}

func (api *nodeLog) Tail(ctx context.Context) io.Reader {
	r, w := io.Pipe()
	go func() {
		defer w.Close() // nolint: errcheck
		<-ctx.Done()
	}()

	writer.WriterGroup.AddWriter(w)

	return r
}

func (api *nodeLog) StreamTo(ctx context.Context, maddr ma.Multiaddr) error {
	nodeDetails, err := api.api.ID().Details()
	if err != nil {
		return err
	}
	peerID := nodeDetails.ID

	// Get the nodes nickname.
	nodeNic, err := api.api.Config().Get("stats.nickname")
	if err != nil {
		return err
	}
	nickname, ok := nodeNic.(string)
	if !ok {
		return errors.New("failed to cast nickname from config")
	}

	mconns, err := multidns.Resolve(ctx, maddr)
	if err != nil {
		return err
	}

	// connection the logs will stream to
	mconn, err := manet.Dial(mconns[0])
	if err != nil {
		return err
	}
	defer mconn.Close() // nolint: errcheck
	wconn := bufio.NewWriter(mconn)

	r, w := io.Pipe()
	go func() {
		defer w.Close() // nolint: errcheck
		defer r.Close() // nolint: errcheck
		<-ctx.Done()
	}()

	// add the pipe to the event log writer group
	writer.WriterGroup.AddWriter(w)

	/*** THIS IS A HACK FOR DEMO ***/
	// Lets make a crappy filter
	filterR, filterW := io.Pipe()
	go func() {
		defer filterR.Close() // nolint: errcheck
		defer filterW.Close() // nolint: errcheck
		<-ctx.Done()
	}()

	// We need this filter to ensure every log message has the peerID and nickname
	filterDecoder := json.NewDecoder(r)
	filterEncoder := json.NewEncoder(filterW)
	go func() {
		defer filterW.Close() // nolint: errcheck
		for {
			if ctx.Err() != nil {
				log.Warningf("filter context error, closing: %v", ctx.Err())
				break
			}
			var event map[string]interface{}
			if err := filterDecoder.Decode(&event); err != nil {
				// We break on error because we don't trust the log writer is still available and expect to be restarted.
				log.Warningf("error decoding event: %v", err)
				break
			}
			// logs that have the field "event" are from a deprecated log method
			// and will be ignored here as they cause a lot of back pressure.
			if event["event"] != nil {
				continue
			}
			// "filter"
			// add things to the event log here
			event["peerName"] = nickname
			event["peerID"] = peerID
			if err := filterEncoder.Encode(event); err != nil {
				log.Warningf("failed to encode event: %v", err)
				break
			}
		}
	}()

	_, err = wconn.ReadFrom(filterR)
	if err != nil {
		return err
	}
	// flush the rest of the events that may be in the pipe before the defered close
	wconn.Flush() // nolint: errcheck

	return nil
}
