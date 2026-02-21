import { useEffect } from "react";

export function useTitle(page: string) {
  useEffect(() => {
    document.title = `${page} — NightOwl`;
    return () => { document.title = "NightOwl — 24/7 Operations Platform"; };
  }, [page]);
}
