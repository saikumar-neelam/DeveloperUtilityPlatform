import type { Metadata } from 'next';
import './globals.css';

export const metadata: Metadata = {
  title: { default: 'WebhookDB', template: '%s · WebhookDB' },
  description: 'Capture, inspect, and replay webhooks in real time.',
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body className="bg-[#F6F9FC] text-slate-900 min-h-screen">
        {children}
      </body>
    </html>
  );
}
