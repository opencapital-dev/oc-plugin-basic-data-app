package plugin

import (
	"context"
	"github.com/opencapital-dev/oc-plugin-sdk/pluginclient"
)

// rwPGClient is the minimal interface App uses from pluginclient.Client.
// Having an interface allows unit tests to inject a fake without a real pgwire connection.
type rwPGClient interface {
	Exec(ctx context.Context, sql string, args ...any) (int64, error)
	Query(ctx context.Context, sql string, args ...any) (pluginclient.Result, error)
	PGExec(ctx context.Context, sql string, args ...any) (int64, error)
	PGQuery(ctx context.Context, sql string, args ...any) (pluginclient.Result, error)
	Config() pluginclient.Config
}
