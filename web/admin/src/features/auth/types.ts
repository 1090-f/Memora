import type { User } from '@/features/user/types';

export interface LoginRequest {
  account: string;
  password: string;
}

export interface LoginResponse {
  access_token: string;
  token_type: string;
  expires_in: number;
  user: User;
}
