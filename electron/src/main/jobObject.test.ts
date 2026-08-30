import type { ChildProcess } from 'node:child_process'
import { describe, expect, it, vi } from 'vitest'
import { assignProcessToJobObject, type NativeJobObject } from './jobObject'

describe('assignProcessToJobObject', () => {
  it('is a no-op on non-Windows platforms (e.g. linux, darwin)', () => {
    const mockJob: NativeJobObject = {
      assignProcess: vi.fn(),
      close: vi.fn(),
    }
    const mockChild = { pid: 1234 } as ChildProcess

    assignProcessToJobObject(mockChild, { platform: 'linux', nativeJobObject: mockJob })
    expect(mockJob.assignProcess).not.toHaveBeenCalled()

    assignProcessToJobObject(mockChild, { platform: 'darwin', nativeJobObject: mockJob })
    expect(mockJob.assignProcess).not.toHaveBeenCalled()
  })

  it('assigns child PID to native job object on win32', () => {
    const mockJob: NativeJobObject = {
      assignProcess: vi.fn(),
      close: vi.fn(),
    }
    const mockChild = { pid: 5678 } as ChildProcess

    assignProcessToJobObject(mockChild, { platform: 'win32', nativeJobObject: mockJob })
    expect(mockJob.assignProcess).toHaveBeenCalledTimes(1)
    expect(mockJob.assignProcess).toHaveBeenCalledWith(5678)
  })

  it('handles child without PID gracefully on win32', () => {
    const mockJob: NativeJobObject = {
      assignProcess: vi.fn(),
      close: vi.fn(),
    }
    const mockChild = {} as ChildProcess

    assignProcessToJobObject(mockChild, { platform: 'win32', nativeJobObject: mockJob })
    expect(mockJob.assignProcess).not.toHaveBeenCalled()
  })

  it('falls back gracefully when nativeJobObject is null on win32 without crashing', () => {
    const mockChild = { pid: 9012 } as ChildProcess
    expect(() => {
      assignProcessToJobObject(mockChild, { platform: 'win32', nativeJobObject: null })
    }).not.toThrow()
  })

  it('catches and logs native addon assign errors without bubbling/crashing', () => {
    const mockJob: NativeJobObject = {
      assignProcess: vi.fn().mockImplementation(() => {
        throw new Error('Access denied in native Job Object')
      }),
      close: vi.fn(),
    }
    const mockChild = { pid: 4321 } as ChildProcess

    expect(() => {
      assignProcessToJobObject(mockChild, { platform: 'win32', nativeJobObject: mockJob })
    }).not.toThrow()
  })
})
