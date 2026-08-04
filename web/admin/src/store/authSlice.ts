import { createSlice, type PayloadAction } from '@reduxjs/toolkit';

export interface AuthState {
  authenticated: boolean;
}

const initialState: AuthState = { authenticated: false };

const authSlice = createSlice({
  name: 'auth',
  initialState,
  reducers: {
    setAuthenticated(state, action: PayloadAction<boolean>) {
      state.authenticated = action.payload;
    },
  },
});

export const { setAuthenticated } = authSlice.actions;
export default authSlice.reducer;
