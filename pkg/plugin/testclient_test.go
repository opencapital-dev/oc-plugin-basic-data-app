package plugin

import (
	"context"

	"github.com/opencapital-dev/oc-plugin-sdk/pluginclient"
)

type fakeClient struct {
	execCalls   []fakeCall
	queryCalls  []fakeCall
	pgExecCalls []fakeCall
	pgQuerCalls []fakeCall

	queryResult   pluginclient.Result
	pgQueryResult pluginclient.Result
}

type fakeCall struct {
	sql  string
	args []any
}

func (f *fakeClient) Exec(ctx context.Context, sql string, args ...any) (int64, error) {
	f.execCalls = append(f.execCalls, fakeCall{sql: sql, args: args})
	return 1, nil
}

func (f *fakeClient) Query(ctx context.Context, sql string, args ...any) (pluginclient.Result, error) {
	f.queryCalls = append(f.queryCalls, fakeCall{sql: sql, args: args})
	return f.queryResult, nil
}

func (f *fakeClient) PGExec(ctx context.Context, sql string, args ...any) (int64, error) {
	f.pgExecCalls = append(f.pgExecCalls, fakeCall{sql: sql, args: args})
	return 1, nil
}

func (f *fakeClient) PGQuery(ctx context.Context, sql string, args ...any) (pluginclient.Result, error) {
	f.pgQuerCalls = append(f.pgQuerCalls, fakeCall{sql: sql, args: args})
	return f.pgQueryResult, nil
}

func (f *fakeClient) Config() pluginclient.Config {
	return pluginclient.Config{PluginID: "test-plugin"}
}
