import { describe, expect, it, vi } from 'vitest'
import backendProcess from './backendProcess'

const { KNOT_DATA_DIR_ENV, buildBackendEnvironment } = backendProcess

describe('buildBackendEnvironment', () => {
  it('passes Electron userData to the Go backend', () => {
    const app = {
      getPath: vi.fn(() => 'C:\\Users\\Test User\\AppData\\Roaming\\Knot')
    }

    const environment = buildBackendEnvironment(app, {
      PATH: 'C:\\Windows\\System32',
      KNOT_DATA_DIR: 'C:\\stale-value'
    })

    expect(app.getPath).toHaveBeenCalledWith('userData')
    expect(environment).toMatchObject({
      PATH: 'C:\\Windows\\System32',
      [KNOT_DATA_DIR_ENV]: 'C:\\Users\\Test User\\AppData\\Roaming\\Knot'
    })
  })
})
