import { deleteRequest, getJson, patchJson, postJson } from './http'

export interface Library {
  id: string
  name: string
  description: string
  allowReaderUploads: boolean
  createdAt: string
  updatedAt: string
}

export interface LibraryMember {
  id: string
  userId: string
  username?: string
  email?: string
  role: 'admin' | 'reader'
  createdAt: string
}

export interface CreateLibraryRequest {
  name: string
  description?: string
  allowReaderUploads?: boolean
}

export interface UpdateLibraryRequest {
  name?: string
  description?: string
  allowReaderUploads?: boolean
}

export interface CreateInvitationRequest {
  email: string
  role: 'admin' | 'reader'
}

export interface InvitationResponse {
  invitationToken: string
  invitationUrl: string
  expiresAt: string
}

export function fetchLibraries(): Promise<{ libraries: Library[] }> {
  return getJson<{ libraries: Library[] }>('/api/v1/libraries')
}

export function fetchLibrary(id: string): Promise<{ library: Library }> {
  return getJson<{ library: Library }>(`/api/v1/libraries/${encodeURIComponent(id)}`)
}

export function createLibrary(data: CreateLibraryRequest): Promise<{ library: Library }> {
  return postJson<{ library: Library }>('/api/v1/libraries', data)
}

export function updateLibrary(id: string, data: UpdateLibraryRequest): Promise<{ library: Library }> {
  return patchJson<{ library: Library }>(`/api/v1/libraries/${encodeURIComponent(id)}`, data)
}

export function deleteLibrary(id: string): Promise<void> {
  return deleteRequest(`/api/v1/libraries/${encodeURIComponent(id)}`)
}

export function fetchLibraryMembers(id: string): Promise<{ members: LibraryMember[] }> {
  return getJson<{ members: LibraryMember[] }>(`/api/v1/libraries/${encodeURIComponent(id)}/members`)
}

export function createLibraryInvitation(id: string, data: CreateInvitationRequest): Promise<InvitationResponse> {
  return postJson<InvitationResponse>(`/api/v1/libraries/${encodeURIComponent(id)}/invitations`, data)
}

export function acceptLibraryInvitation(token: string): Promise<{ libraryId: string; role: string }> {
  return postJson<{ libraryId: string; role: string }>(`/api/v1/invitations/${encodeURIComponent(token)}/accept`)
}
