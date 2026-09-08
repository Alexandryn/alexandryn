import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import React, { useState } from 'react'
import { Button } from '../../components/Button'
import {
  createLibrary,
  createLibraryInvitation,
  fetchLibraries,
  fetchLibraryMembers,
  type Library,
  type LibraryMember,
} from '../../data/libraries'

export function LibraryManagement() {
  const queryClient = useQueryClient()
  const [selectedLibId, setSelectedLibId] = useState<string | null>(null)
  const [isCreatingLib, setIsCreatingLib] = useState(false)
  const [newLibName, setNewLibName] = useState('')
  const [newLibDesc, setNewLibDesc] = useState('')
  const [allowUploads, setAllowUploads] = useState(false)

  const [inviteEmail, setInviteEmail] = useState('')
  const [inviteRole, setInviteRole] = useState<'reader' | 'admin'>('reader')
  const [inviteResult, setInviteResult] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  const { data: libsData, isLoading: libsLoading } = useQuery({
    queryKey: ['libraries'],
    queryFn: fetchLibraries,
  })

  const libraries = libsData?.libraries || []
  const activeLibId = selectedLibId || (libraries[0]?.id ?? null)

  const { data: membersData } = useQuery({
    queryKey: ['library-members', activeLibId],
    queryFn: () => (activeLibId ? fetchLibraryMembers(activeLibId) : Promise.resolve({ members: [] })),
    enabled: !!activeLibId,
  })

  const createLibMutation = useMutation({
    mutationFn: createLibrary,
    onSuccess: (res) => {
      queryClient.invalidateQueries({ queryKey: ['libraries'] })
      setIsCreatingLib(false)
      setNewLibName('')
      setNewLibDesc('')
      setAllowUploads(false)
      setSelectedLibId(res.library.id)
    },
    onError: (err) => setError(err instanceof Error ? err.message : 'Failed to create library'),
  })

  const createInviteMutation = useMutation({
    mutationFn: ({ libId, data }: { libId: string; data: { email: string; role: 'reader' | 'admin' } }) =>
      createLibraryInvitation(libId, data),
    onSuccess: (res) => {
      setInviteResult(window.location.origin + res.invitationUrl)
      setInviteEmail('')
    },
    onError: (err) => setError(err instanceof Error ? err.message : 'Failed to generate invitation'),
  })

  const handleCreateLib = (e: React.FormEvent) => {
    e.preventDefault()
    if (!newLibName.trim()) return
    setError(null)
    createLibMutation.mutate({
      name: newLibName.trim(),
      description: newLibDesc.trim(),
      allowReaderUploads: allowUploads,
    })
  }

  const handleCreateInvite = (e: React.FormEvent) => {
    e.preventDefault()
    if (!activeLibId || !inviteEmail.trim()) return
    setError(null)
    setInviteResult(null)
    createInviteMutation.mutate({
      libId: activeLibId,
      data: { email: inviteEmail.trim(), role: inviteRole },
    })
  }

  if (libsLoading) {
    return <div className="p-xl text-center text-text-3">Loading libraries...</div>
  }

  return (
    <div className="p-lg max-w-5xl mx-auto flex flex-col gap-xl">
      <div className="flex justify-between items-center border-b border-border pb-md">
        <div>
          <h1 className="text-2xl font-serif font-bold text-text">Library Namespaces</h1>
          <p className="text-sm text-text-3">Manage library partitions, ingestion permissions, and access memberships.</p>
        </div>
        <Button onClick={() => setIsCreatingLib(!isCreatingLib)}>
          {isCreatingLib ? 'Cancel' : 'Create Library'}
        </Button>
      </div>

      {error && (
        <div className="rounded border border-error bg-surface p-md text-sm text-error" role="alert">
          {error}
        </div>
      )}

      {isCreatingLib && (
        <form onSubmit={handleCreateLib} className="p-lg bg-surface border border-border rounded-lg flex flex-col gap-md">
          <h2 className="text-lg font-medium text-text">New Library Namespace</h2>
          <div className="flex flex-col gap-xs">
            <label className="text-sm font-medium text-text-2" htmlFor="libName">Name</label>
            <input
              id="libName"
              type="text"
              required
              value={newLibName}
              onChange={(e) => setNewLibName(e.target.value)}
              className="px-md py-sm bg-background border border-border rounded text-text"
              placeholder="e.g. Manga & Comics"
            />
          </div>
          <div className="flex flex-col gap-xs">
            <label className="text-sm font-medium text-text-2" htmlFor="libDesc">Description</label>
            <input
              id="libDesc"
              type="text"
              value={newLibDesc}
              onChange={(e) => setNewLibDesc(e.target.value)}
              className="px-md py-sm bg-background border border-border rounded text-text"
              placeholder="Optional description"
            />
          </div>
          <div className="flex items-center gap-sm">
            <input
              id="allowUploads"
              type="checkbox"
              checked={allowUploads}
              onChange={(e) => setAllowUploads(e.target.checked)}
              className="rounded border-border text-accent"
            />
            <label htmlFor="allowUploads" className="text-sm text-text-2">
              Allow Readers to import/upload books to this library
            </label>
          </div>
          <div className="flex justify-end gap-sm mt-sm">
            <Button type="submit" disabled={createLibMutation.isPending}>
              {createLibMutation.isPending ? 'Creating...' : 'Create'}
            </Button>
          </div>
        </form>
      )}

      <div className="grid grid-cols-1 md:grid-cols-3 gap-lg">
        <div className="flex flex-col gap-sm">
          <h2 className="text-base font-semibold text-text mb-xs">Libraries</h2>
          {libraries.map((lib: Library) => (
            <button
              key={lib.id}
              onClick={() => {
                setSelectedLibId(lib.id)
                setInviteResult(null)
              }}
              className={`p-md rounded-lg text-left border transition-colors ${
                lib.id === activeLibId
                  ? 'bg-accent/10 border-accent text-accent font-medium'
                  : 'bg-surface border-border text-text hover:border-border-muted'
              }`}
            >
              <div className="font-medium text-sm">{lib.name}</div>
              {lib.description && <div className="text-xs text-text-3 truncate mt-0.5">{lib.description}</div>}
              <div className="text-xs text-text-3 mt-1.5 flex items-center gap-1">
                <span className={`w-1.5 h-1.5 rounded-full ${lib.allowReaderUploads ? 'bg-success' : 'bg-warm'}`} />
                {lib.allowReaderUploads ? 'Reader uploads enabled' : 'Admin only uploads'}
              </div>
            </button>
          ))}
        </div>

        <div className="md:col-span-2 flex flex-col gap-lg bg-surface p-lg rounded-lg border border-border">
          <div>
            <h2 className="text-lg font-medium text-text mb-xs">Members & Access</h2>
            <p className="text-xs text-text-3">Users with access to this library aggregate partition.</p>
          </div>

          <div className="flex flex-col gap-xs">
            {(membersData?.members || []).map((m: LibraryMember) => (
              <div key={m.id} className="p-sm flex justify-between items-center bg-background rounded border border-border text-xs">
                <div>
                  <span className="font-medium text-text">{m.username || m.userId}</span>
                  {m.email && <span className="text-text-3 ml-sm">({m.email})</span>}
                </div>
                <span className="px-sm py-0.5 rounded bg-surface text-text-2 uppercase font-semibold text-2xs">
                  {m.role}
                </span>
              </div>
            ))}
          </div>

          <form onSubmit={handleCreateInvite} className="pt-md border-t border-border flex flex-col gap-md">
            <h3 className="text-sm font-semibold text-text">Invite Member</h3>
            <div className="flex flex-col sm:flex-row gap-sm">
              <input
                type="email"
                required
                value={inviteEmail}
                onChange={(e) => setInviteEmail(e.target.value)}
                className="flex-1 px-md py-sm bg-background border border-border rounded text-sm text-text"
                placeholder="friend@example.com"
              />
              <select
                value={inviteRole}
                onChange={(e) => setInviteRole(e.target.value as 'reader' | 'admin')}
                className="px-md py-sm bg-background border border-border rounded text-sm text-text"
              >
                <option value="reader">Reader</option>
                <option value="admin">Admin</option>
              </select>
              <Button type="submit" disabled={createInviteMutation.isPending}>
                {createInviteMutation.isPending ? 'Generating...' : 'Invite'}
              </Button>
            </div>

            {inviteResult && (
              <div className="rounded border border-success bg-surface p-md text-xs text-success flex flex-col gap-xs">
                <span className="font-semibold">Invitation Link Generated:</span>
                <code className="select-all break-all bg-surface-3 p-xs rounded font-mono">{inviteResult}</code>
              </div>
            )}
          </form>
        </div>
      </div>
    </div>
  )
}
