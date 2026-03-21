import { Sidebar } from '@/components/Sidebar';

export default function EndpointsLayout({ children }: { children: React.ReactNode }) {
  return (
    <>
      <Sidebar />
      <div className="ml-[240px] min-h-screen flex flex-col">
        {children}
      </div>
    </>
  );
}
