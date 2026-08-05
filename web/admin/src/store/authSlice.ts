import { createSlice, type PayloadAction } from '@reduxjs/toolkit';
import type { User } from '@/features/user/types';

export interface AuthState {
  authenticated: boolean;
  user: User | null;
}

const initialState: AuthState = { authenticated: false, user: null };

const authSlice = createSlice({
  name: 'auth',
  initialState,
  reducers: {
    setAuthenticated(state, action: PayloadAction<boolean>) {
      state.authenticated = action.payload;
    },
    setAuthSession(state, action: PayloadAction<User>) {
      state.authenticated = true;
      state.user = action.payload;
    },
    updateAuthUser(state, action: PayloadAction<User>) {
      state.user = action.payload;
    },
    clearAuth(state) {
      state.authenticated = false;
      state.user = null;
    },
  },
});

export const { setAuthenticated, setAuthSession, updateAuthUser, clearAuth } =
  authSlice.actions;
export default authSlice.reducer;
