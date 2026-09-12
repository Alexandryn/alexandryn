import { expect, test } from '@playwright/test'

// Performance verification: a grid of 500 generated covers renders
// its initial viewport within 100ms of layout data being available, and
// scrolling stays at 60fps.

test('renders the initial viewport of 500 generated covers within budget', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByTestId('cover-grid')).toBeVisible()

  // The measure is written inside a double-rAF callback (main.tsx) so it
  // reflects real paint, not just React's commit — wait for it to exist
  // rather than assuming it's already there once the container is visible.
  await page.waitForFunction(
    () => performance.getEntriesByName('cover-grid:initial-viewport').length > 0,
  )

  const duration = await page.evaluate(() => {
    return performance.getEntriesByName('cover-grid:initial-viewport')[0]!.duration
  })

  expect(duration).toBeLessThanOrEqual(100)
})

test('scrolling through 500 generated covers stays close to 60fps', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByTestId('cover-grid')).toBeVisible()

  const frameIntervals = await page.evaluate(async () => {
    const el = document.querySelector('[data-testid="cover-grid"]') as HTMLElement
    const samples: number[] = []
    let last = performance.now()
    return await new Promise<number[]>((resolve) => {
      let frames = 0
      const totalFrames = 60
      function step(now: number) {
        samples.push(now - last)
        last = now
        el.scrollTop += 40
        frames++
        if (frames < totalFrames) requestAnimationFrame(step)
        else resolve(samples)
      }
      requestAnimationFrame(step)
    })
  })

  // Drop the first sample (includes the wait for the first rAF callback
  // itself, not a real frame-to-frame interval).
  const intervals = frameIntervals.slice(1)
  intervals.sort((a, b) => a - b)
  const median = intervals[Math.floor(intervals.length / 2)]!

  // 60fps = ~16.7ms/frame. Median (not mean, to avoid one slow CI-runner
  // frame skewing the result) budgeted with slack for headless-runner
  // scheduling jitter, still tight enough to catch a real regression.
  expect(median).toBeLessThanOrEqual(20)
})
