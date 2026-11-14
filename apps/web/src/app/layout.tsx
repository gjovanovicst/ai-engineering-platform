import type { Metadata } from "next";
import "./globals.css";
import { Providers } from "./providers";

export const metadata: Metadata = {
  title: "AI Engineering Platform",
  description: "AI agents executing the software lifecycle",
  icons: { icon: "/icons/icon.svg", apple: "/icons/icon.svg" },
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" suppressHydrationWarning>
      <body>
        <script
          // Blocking theme bootstrap: set the class before the first paint so a
          // dark-theme reload never flashes the light palette.
          dangerouslySetInnerHTML={{
            __html: `(function(){try{var t=localStorage.getItem('aip-theme');var d=t==='dark'||(t!=='light'&&window.matchMedia&&window.matchMedia('(prefers-color-scheme: dark)').matches);var e=document.documentElement;if(d){e.classList.add('dark')}e.style.colorScheme=d?'dark':'light'}catch(e){}})();`,
          }}
        />
        <Providers>
          {process.env.NODE_ENV === "development" ? (
            <script
              // Development only: drop a stale service worker / CacheStorage
              // left on localhost:3000, then reload once. Runs before hydration.
              dangerouslySetInnerHTML={{
                __html: `(function(){if(!('serviceWorker' in navigator))return;navigator.serviceWorker.getRegistrations().then(function(rs){if(!rs.length)return;Promise.all(rs.map(function(r){return r.unregister()})).then(function(){if('caches' in window){caches.keys().then(function(ks){return Promise.all(ks.map(function(k){return caches.delete(k)}))})}if(!sessionStorage.getItem('aip-cache-reset')){sessionStorage.setItem('aip-cache-reset','1');location.reload()}})})})();`,
              }}
            />
          ) : null}
          {children}
        </Providers>
      </body>
    </html>
  );
}

export const viewport = {
  themeColor: "#4f46e5",
};
