import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { SpeedInsights } from '@vercel/speed-insights/react';
import AppController from './Controllers/AppController';
import './Views/styles.scss';

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <AppController />
    <SpeedInsights />
  </StrictMode>,
);
