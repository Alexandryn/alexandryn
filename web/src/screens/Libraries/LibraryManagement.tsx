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
    return <div className="p-xl text-center text-text-muted">Loading libraries...</div>
  }

  return (
    <div className="p-lg max-w-5xl mx-auto flex flex-col gap-xl">
      <div className="flex justify-between items-center border-b border-border-subtle pb-md">
        <div>
          <h1 className="text-2xl font-serif font-bold text-text-primary">Library Namespaces</h1>
          <p className="text-sm text-text-muted">Manage library partitions, ingestion permissions, and access memberships.</p>
        </div>
        <Button onClick={() => setIsCreatingLib(!isCreatingLib)}>
          {isCreatingLib ? 'Cancel' : 'Create Library'}
        </Button>
      </div>

      {error && (
        <div className="p-md rounded bg-red-950/40 border border-red-800 text-red-300 text-sm" role="alert">
          {error}
        </div>
      )}

      {isCreatingLib && (
        <form onSubmit={handleCreateLib} className="p-lg bg-bg-surface border border-border-subtle rounded-lg flex flex-col gap-md">
          <h2 className="text-lg font-medium text-text-primary">New Library Namespace</h2>
          <div className="flex flex-col gap-xs">
            <label className="text-sm font-medium text-text-secondary" htmlFor="libName">Name</label>
            <input
              id="libName"
              type="text"
              required
              value={newLibName}
              onChange={(e) => setNewLibName(e.target.value)}
              className="px-md py-sm bg-bg-canvas border border-border-subtle rounded text-text-primary"
              placeholder="e.g. Manga & Comics"
            />
          </div>
          <div className="flex flex-col gap-xs">
            <label className="text-sm font-medium text-text-secondary" htmlFor="libDesc">Description</label>
            <input
              id="libDesc"
              type="text"
              value={newLibDesc}
              onChange={(e) => setNewLibDesc(e.target.value)}
              className="px-md py-sm bg-bg-canvas border border-border-subtle rounded text-text-primary"
              placeholder="Optional description"
            />
          </div>
          <div className="flex items-center gap-sm">
            <input
              id="allowUploads"
              type="checkbox"
              checked={allowUploads}
              onChange={(e) => setAllowUploads(e.target.checked)}
              className="rounded border-border-subtle text-accent"
            />
            <label htmlFor="allowUploads" className="text-sm text-text-secondary">
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
          <h2 className="text-base font-semibold text-text-primary mb-xs">Libraries</h2>
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
                  : 'bg-bg-surface border-border-subtle text-text-primary hover:border-border-muted'
              }`}
            >
              <div className="font-medium text-sm">{lib.name}</div>
              {lib.description && <div className="text-xs text-text-muted truncate mt-0.5">{lib.description}</div>}
              <div className="text-xs text-text-muted mt-1.5 flex items-center gap-1">
                <span className={`w-1.5 h-1.5 rounded-full ${lib.allowReaderUploads ? 'bg-emerald-500' : 'bg-amber-500'}`} />
                {lib.allowReaderUploads ? 'Reader uploads enabled' : 'Admin only uploads'}
              </div>
            </button>
          ))}
        </div>

        <div className="md:col-span-2 flex flex-col gap-lg bg-bg-surface p-lg rounded-lg border border-border-subtle">
          <div>
            <h2 className="text-lg font-medium text-text-primary mb-xs">Members & Access</h2>
            <p className="text-xs text-text-muted">Users with access to this library aggregate partition.</p>
          </div>

          <div className="flex flex-col gap-xs">
            {(membersData?.members || []).map((m: LibraryMember) => (
              <div key={m.id} className="p-sm flex justify-between items-center bg-bg-canvas rounded border border-border-subtle text-xs">
                <div>
                  <span className="font-medium text-text-primary">{m.username || m.userId}</span>
                  {m.email && <span className="text-text-muted ml-sm">({m.email})</span>}
                </div>
                <span className="px-sm py-0.5 rounded bg-bg-surface text-text-secondary uppercase font-semibold text-[10px]">
                  {m.role}
                </span>
              </div>
            ))}
          </div>

          <form onSubmit={handleCreateInvite} className="pt-md border-t border-border-subtle flex flex-col gap-md">
            <h3 className="text-sm font-semibold text-text-primary">Invite Member</h3>
            <div className="flex flex-col sm:flex-row gap-sm">
              <input
                type="email"
                required
                value={inviteEmail}
                onChange={(e) => setInviteEmail(e.target.value)}
                className="flex-1 px-md py-sm bg-bg-canvas border border-border-subtle rounded text-sm text-text-primary"
                placeholder="friend@example.com"
              />
              <select
                value={inviteRole}
                onChange={(e) => setInviteRole(e.target.value as 'reader' | 'admin')}
                className="px-md py-sm bg-bg-canvas border border-border-subtle rounded text-sm text-text-primary"
              >
                <option value="reader">Reader</option>
                <option value="admin">Admin</option>
              </select>
              <Button type="submit" disabled={createInviteMutation.isPending}>
                {createInviteMutation.isPending ? 'Generating...' : 'Invite'}
              </Button>
            </div>

            {inviteResult && (
              <div className="p-md rounded bg-emerald-950/40 border border-emerald-800 text-emerald-300 text-xs flex flex-col gap-xs">
                <span className="font-semibold">Invitation Link Generated:</span>
                <code className="select-all break-all bg-black/40 p-xs rounded font-mono">{inviteResult}</code>
              </div>
            )}
          </form>
        </div>
      </div>
    </div>
  )
}
