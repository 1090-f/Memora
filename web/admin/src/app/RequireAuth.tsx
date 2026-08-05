import { Navigate, Outlet, useLocation } from 'react-router-dom';
import { useAppSelector } from '@/store';

export function RequireAuth() {
  const authenticated = useAppSelector((state) => state.auth.authenticated);
  const location = useLocation();

  if (!authenticated) {
    const redirect = encodeURIComponent(location.pathname + location.search);
    return <Navigate to={`/login?redirect=${redirect}`} replace />;
  }

  return <Outlet />;
}
