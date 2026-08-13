"use client";

import { useCallback, useEffect, useState } from "react";
import { listServices, type ServiceSummary } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { myBookings, type Booking } from "@/lib/bookings";

/**
 * Loads the client's bookings + the service catalog (to resolve service
 * names) for the portal views. `bookings` stays null only on the first load
 * (skeleton state); `refresh()` re-fetches in place after mutations so the
 * list never flashes back to skeletons.
 */
export function useMyBookings() {
  const { session, onTokensRefreshed } = useAuth();
  const [bookings, setBookings] = useState<Booking[] | null>(null);
  const [servicesById, setServicesById] = useState<Map<string, ServiceSummary>>(
    new Map(),
  );
  const [error, setError] = useState<string | null>(null);
  const [refreshIndex, setRefreshIndex] = useState(0);

  useEffect(() => {
    if (!session) return;
    let cancelled = false;
    Promise.all([myBookings(session, { onTokensRefreshed }), listServices()])
      .then(([items, services]) => {
        if (cancelled) return;
        setBookings(items);
        setServicesById(new Map(services.map((service) => [service.id, service])));
        setError(null);
      })
      .catch(() => {
        if (!cancelled) {
          setError("Your sessions didn't load. Check your connection and try again.");
        }
      });
    return () => {
      cancelled = true;
    };
  }, [session, onTokensRefreshed, refreshIndex]);

  const refresh = useCallback(() => setRefreshIndex((index) => index + 1), []);

  return { bookings, servicesById, error, refresh };
}
