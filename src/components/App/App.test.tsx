import React from 'react';
import { MemoryRouter } from 'react-router-dom';
import { AppRootProps, PluginType } from '@grafana/data';
import { render, waitFor } from '@testing-library/react';
import App from './App';

// Mock @grafana/runtime so the pages' getBackendSrv() / getDataSourceSrv()
// don't reach into a non-existent Grafana stack during unit tests.
jest.mock('@grafana/runtime', () => ({
  PluginPage: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  getBackendSrv: () => ({
    get: jest.fn().mockResolvedValue([]),
    post: jest.fn().mockResolvedValue({}),
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
        id: 'yfinance-app',
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

  test('renders the Instruments route by default', async () => {
    const { queryAllByText } = render(
      <MemoryRouter initialEntries={['/instruments']}>
        <App {...props} />
      </MemoryRouter>
    );
    // The Instruments page renders a heading or empty-state we can wait on.
    await waitFor(() => expect(queryAllByText(/instruments/i).length).toBeGreaterThan(0), {
      timeout: 2000,
    });
  });
});
