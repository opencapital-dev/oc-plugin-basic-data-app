import React from 'react';
import { render, screen } from '@testing-library/react';
import { PluginType } from '@grafana/data';
import AppConfig, { AppConfigProps } from './AppConfig';

describe('Components/AppConfig', () => {
  let props: AppConfigProps;

  beforeEach(() => {
    jest.resetAllMocks();
    props = {
      plugin: {
        meta: {
          id: 'basic-data-app',
          name: 'yFinance Data',
          type: PluginType.app,
          enabled: true,
          jsonData: {},
        },
      },
      query: {},
    } as unknown as AppConfigProps;
  });

  test('renders the read-only info panel', () => {
    // @ts-ignore - addConfigPage() / setChannelSupport() not needed in tests
    render(<AppConfig plugin={props.plugin} query={props.query} />);
    expect(screen.getByText(/yFinance Data/i)).toBeInTheDocument();
    expect(screen.getByText(/basic-data-app_url/i)).toBeInTheDocument();
  });
});
