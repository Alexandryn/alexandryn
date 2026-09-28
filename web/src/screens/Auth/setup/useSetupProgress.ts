import { useState, useEffect, useCallback, useRef } from 'react'

export type SetupStep =
  | 'welcome'
  | 'admin'
  | 'network'
  | 'services'
  | 'commands'
  | 'verification'
  | 'complete'

export const SETUP_STEPS: SetupStep[] = [
  'welcome',
  'admin',
  'network',
  'services',
  'commands',
  'verification',
  'complete',
]

const STORAGE_KEY = 'alexandryn_setup_progress'

export interface SetupProgressData {
  step: SetupStep
  networkMode: 'lan' | 'loopback'
  hostName: string
  completedSteps: SetupStep[]
  commandStatus: Record<string, 'pending' | 'executing' | 'verified' | 'failed'>
}

const DEFAULT_STATE: SetupProgressData = {
  step: 'welcome',
  networkMode: 'lan',
  hostName: 'alexandryn.local',
  completedSteps: [],
  commandStatus: {},
}

function loadSavedState(): SetupProgressData {
  if (typeof window === 'undefined') return DEFAULT_STATE
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return DEFAULT_STATE
    const parsed = JSON.parse(raw) as Partial<SetupProgressData>
    return {
      step: parsed.step && SETUP_STEPS.includes(parsed.step) ? parsed.step : 'welcome',
      networkMode: parsed.networkMode === 'loopback' ? 'loopback' : 'lan',
      hostName: parsed.hostName || 'alexandryn.local',
      completedSteps: Array.isArray(parsed.completedSteps) ? parsed.completedSteps : [],
      commandStatus: parsed.commandStatus || {},
    }
  } catch {
    return DEFAULT_STATE
  }
}

export function useSetupProgress(backendIsSetup = false) {
  const [state, setState] = useState<SetupProgressData>(() => loadSavedState())
  const isFinishedRef = useRef(false)

  // If backend already has admin initialized and user is at admin step, advance to network
  const step: SetupStep =
    backendIsSetup && state.step === 'admin'
      ? 'network'
      : state.step

  // Persist state changes to localStorage
  useEffect(() => {
    if (typeof window !== 'undefined' && !isFinishedRef.current) {
      try {
        localStorage.setItem(STORAGE_KEY, JSON.stringify(state))
      } catch {
        // Ignore storage errors (e.g. storage quota / disabled)
      }
    }
  }, [state])

  const setStep = useCallback((step: SetupStep) => {
    setState((prev) => ({ ...prev, step }))
  }, [])

  const markStepComplete = useCallback((step: SetupStep) => {
    setState((prev) => ({
      ...prev,
      completedSteps: Array.from(new Set([...prev.completedSteps, step])),
    }))
  }, [])

  const setNetworkMode = useCallback((mode: 'lan' | 'loopback') => {
    setState((prev) => ({ ...prev, networkMode: mode }))
  }, [])

  const setHostName = useCallback((hostName: string) => {
    setState((prev) => ({ ...prev, hostName }))
  }, [])

  const setCommandStatus = useCallback(
    (key: string, status: 'pending' | 'executing' | 'verified' | 'failed') => {
      setState((prev) => ({
        ...prev,
        commandStatus: { ...prev.commandStatus, [key]: status },
      }))
    },
    [],
  )

  const resetProgress = useCallback(() => {
    isFinishedRef.current = true
    if (typeof window !== 'undefined') {
      localStorage.removeItem(STORAGE_KEY)
    }
    setState(DEFAULT_STATE)
  }, [])

  const isStepComplete = useCallback(
    (step: SetupStep) => state.completedSteps.includes(step),
    [state.completedSteps],
  )

  return {
    step,
    setStep,
    networkMode: state.networkMode,
    setNetworkMode,
    hostName: state.hostName,
    setHostName,
    completedSteps: state.completedSteps,
    markStepComplete,
    isStepComplete,
    commandStatus: state.commandStatus,
    setCommandStatus,
    resetProgress,
  }
}
