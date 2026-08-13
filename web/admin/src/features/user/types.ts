export interface User {
  id: string;
  username: string;
  nickname: string;
  email: string;
  avatar_url: string | null;
  bio?: string | null;
}

export interface UpdateCurrentUserInput {
  nickname?: string;
  bio?: string;
  email?: string;
}

export interface ChangePasswordInput {
  old_password: string;
  new_password: string;
}
