import { configureStore } from '@reduxjs/toolkit';
import { type TypedUseSelectorHook, useDispatch, useSelector } from 'react-redux';
import { readStoredSession } from '@/features/auth/session';
import auth from './authSlice';

export const createAppStore = () =>
  configureStore({
    reducer: { auth },
    preloadedState: {
      auth: { authenticated: readStoredSession() !== null },
    },
  });

export type AppStore = ReturnType<typeof createAppStore>;
export type RootState = ReturnType<AppStore['getState']>;
export type AppDispatch = AppStore['dispatch'];

export const useAppDispatch = useDispatch.withTypes<AppDispatch>();
export const useAppSelector: TypedUseSelectorHook<RootState> = useSelector;
