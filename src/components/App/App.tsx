import React from 'react';
import { Navigate, Route, Routes } from 'react-router-dom';
import { type AppRootProps } from '@grafana/data';

import { ROUTES } from '../../constants';
import { InstrumentsPage } from '../../pages/InstrumentsPage';

function App(_props: AppRootProps) {
  return (
    <Routes>
      <Route path={ROUTES.Instruments} element={<InstrumentsPage />} />
      <Route path="*" element={<Navigate replace to={ROUTES.Instruments} />} />
    </Routes>
  );
}

export default App;
