package server

import (
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/server/search"
	"github.com/drausin/libri/libri/librarian/server/store"
)

const (
	logSelfIDShort     = "self_id_short"
	logRequestIDShort  = "request_id_short"
	logFromPubKeyShort = "from_pub_key_short"
	logNPeers          = "n_peers"
	logKey             = "key"
	logOperation       = "operation"
	logNReplicas       = "n_replicas"
	logSearch          = "search"
	logStore           = "store"
)

func rqMetadataFields(md *api.RequestMetadata) []zapcore.Field {
	return []zapcore.Field{
		zap.String(logRequestIDShort, id.ShortHex(md.RequestId)),
		zap.String(logFromPubKeyShort, id.ShortHex(md.PubKey[1:9])),
	}
}

func introduceRequestFields(rq *api.IntroduceRequest) []zapcore.Field {
	return []zapcore.Field{
		zap.Uint32(logNPeers, rq.NumPeers),
	}
}

func introduceResponseFields(rp *api.IntroduceResponse) []zapcore.Field {
	return []zapcore.Field{
		zap.Int(logNPeers, len(rp.Peers)),
	}
}

func findRequestFields(rq *api.FindRequest) []zapcore.Field {
	return []zapcore.Field{
		zap.String(logKey, id.Hex(rq.Key)),
		zap.Uint32(logNPeers, rq.NumPeers),
	}
}

func findValueResponseFields(rq *api.FindRequest, _ *api.FindResponse) []zapcore.Field {
	return []zapcore.Field{
		zap.String(logKey, id.Hex(rq.Key)),
	}
}

func findPeersResponseFields(rq *api.FindRequest, rp *api.FindResponse) []zapcore.Field {
	return []zapcore.Field{
		zap.String(logKey, id.Hex(rq.Key)),
		zap.Int(logNPeers, len(rp.Peers)),
	}
}

func storeRequestFields(rq *api.StoreRequest) []zapcore.Field {
	return []zapcore.Field{
		zap.String(logKey, id.Hex(rq.Key)),
	}
}

func storeResponseFields(rq *api.StoreRequest, _ *api.StoreResponse) []zapcore.Field {
	return []zapcore.Field{
		zap.String(logKey, id.Hex(rq.Key)),
	}
}

func getRequestFields(rq *api.GetRequest) []zapcore.Field {
	return []zapcore.Field{
		zap.String(logKey, id.Hex(rq.Key)),
	}
}

func getResponseFields(rq *api.GetRequest, _ *api.GetResponse) []zapcore.Field {
	return []zapcore.Field{
		zap.String(logKey, id.Hex(rq.Key)),
	}
}

func searchDetailFields(s *search.Search) []zapcore.Field {
	return []zapcore.Field{
		zap.Object(logSearch, s),
	}
}

func putRequestFields(rq *api.PutRequest) []zapcore.Field {
	return []zapcore.Field{
		zap.String(logKey, id.Hex(rq.Key)),
	}
}

func putResponseFields(rq *api.PutRequest, rp *api.PutResponse) []zapcore.Field {
	return []zapcore.Field{
		zap.String(logKey, id.Hex(rq.Key)),
		zap.Stringer(logOperation, rp.Operation),
		zap.Uint32(logNReplicas, rp.NReplicas),
	}
}

func storeDetailFields(s *store.Store) []zapcore.Field {
	return []zapcore.Field{
		zap.Object(logStore, s),
	}
}
