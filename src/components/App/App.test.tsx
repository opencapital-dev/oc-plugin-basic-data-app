import React from 'react';
import { MemoryRouter } from 'react-router-dom';
import { AppRootProps, PluginType } from '@grafana/data';
import { render, screen, waitFor } from '@testing-library/react';
import App from './App';

// Mock @grafana/runtime so the pages' getBackendSrv() / getDataSourceSrv()
// don't reach into a non-existent Grafana stack during unit tests.
jest.mock('@grafana/runtime', () => ({
  PluginPage: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  // Per-URL mock: instruments endpoints return arrays, settings returns an object.
  getBackendSrv: () => ({
    get: jest.fn().mockImplementation((url: string) => {
      if (url.includes('/settings')) {
        return Promise.resolve({ fred_api_key_set: false, pollIntervalSec: 15, qps: 1, burst: 3, liveEnable: true, backfillEnable: true });
      }
      return Promise.resolve([]);
    }),
    post: jest.fn().mockResolvedValue({}),
    put: jest.fn().mockResolvedValue({ ok: true }),
  }),
  getAppEvents: () => ({ publish: jest.fn() }),
}));

describe('Components/App', () => {
  let props: AppRootProps;

  beforeEach(() => {
    jest.resetAllMocks();
    props = {
      basename: 'a/yfinance-ingestor',
      meta: {
        id: 'basic-data-app',
        name: 'yFinance Data',
        type: PluginType.app,
        enabled: true,
        jsonData: {},
      },
      query: {},
      path: '',
      onNavChanged: jest.fn(),
    } as unknown as AppRootProps;
  });

  test('renders the Tickers route by default', async () => {
    const { queryAllByText } = render(
      <MemoryRouter initialEntries={['/tickers']}>
        <App {...props} />
      </MemoryRouter>
    );
    // The Tickers page renders a heading or empty-state we can wait on.
    await waitFor(() => expect(queryAllByText(/instruments|tickers|Yahoo/i).length).toBeGreaterThan(0), {
      timeout: 2000,
    });
  });

  test('renders the Settings page at the settings route', async () => {
    render(
      <MemoryRouter initialEntries={['/settings']}>
        <App {...props} />
      </MemoryRouter>
    );
    expect(await screen.findByText(/FRED API key/i)).toBeInTheDocument();
  });
});
