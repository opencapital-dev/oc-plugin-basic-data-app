import React, { useEffect, useState } from 'react';
import { Alert, Button, Field, SecretInput } from '@grafana/ui';

import { getSettings, putSettings, testFred, type Settings } from '../api/settings';

export function SettingsPage() {
  const [s, setS] = useState<Settings | null>(null);
  const [key, setKey] = useState('');
  const [msg, setMsg] = useState<string | null>(null);

  useEffect(() => {
    getSettings().then(setS);
  }, []);

  const save = async () => {
    await putSettings({ fred_api_key: key });
    setMsg('Saved');
    setKey('');
    getSettings().then(setS);
  };

  const test = async () => {
    const r = await testFred();
    setMsg(r.ok ? 'FRED key OK' : 'FRED key invalid');
  };

  return (
    <div>
      <h2>Settings</h2>
      <Field label="FRED API key" description="Get one at fredaccount.stlouisfed.org. Stored locally.">
        <SecretInput
          isConfigured={!!s?.fred_api_key_set}
          value={key}
          placeholder={s?.fred_api_key_set ? 'configured' : 'enter key'}
          onChange={(e) => setKey(e.currentTarget.value)}
          onReset={() => setKey('')}
        />
      </Field>
      <Button onClick={save}>Save</Button>{' '}
      <Button variant="secondary" onClick={test}>
        Test
      </Button>
      {msg && <Alert title={msg} severity="info" />}
    </div>
  );
}
