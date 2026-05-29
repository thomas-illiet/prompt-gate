export type AppRole = 'none' | 'user' | 'manager' | 'admin'

export interface AuthUser {
  id: string
  sub: string
  preferredUsername: string
  email: string
  name: string
  role: AppRole
  isActive: boolean
  lastLoginAt: string
}
