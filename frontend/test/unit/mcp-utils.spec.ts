import { describe, expect, it } from 'vitest'

import {
  buildMCPHeaderPayloads,
  buildMCPServerPayload,
  findDuplicateMCPHeaderNames,
  isValidMCPHeaderName,
  isValidMCPServerName,
  isValidMCPURL,
} from '../../app/utils/mcp'

describe('mcp utils', () => {
  it('validates server names and URLs', () => {
    expect(isValidMCPServerName('linear-tools')).toBe(true)
    expect(isValidMCPServerName(' Linear-Tools ')).toBe(true)
    expect(isValidMCPServerName('linear_tools')).toBe(false)
    expect(isValidMCPURL('https://mcp.example.com/mcp')).toBe(true)
    expect(isValidMCPURL('ftp://mcp.example.com')).toBe(false)
  })

  it('validates header names and detects duplicates case-insensitively', () => {
    const rows = [
      {
        id: 'one',
        name: 'Authorization',
        value: '',
        sensitive: true,
        hasValue: true,
        clearValue: false,
      },
      {
        id: 'two',
        name: 'authorization',
        value: '',
        sensitive: true,
        hasValue: false,
        clearValue: false,
      },
    ]

    expect(isValidMCPHeaderName('X-Api-Key')).toBe(true)
    expect(isValidMCPHeaderName('X Api Key')).toBe(false)
    expect(findDuplicateMCPHeaderNames(rows)).toEqual(new Set(['authorization']))
  })

  it('omits unchanged sensitive values and sends null when cleared', () => {
    expect(
      buildMCPHeaderPayloads([
        {
          id: 'stored',
          name: 'Authorization',
          value: '',
          sensitive: true,
          hasValue: true,
          clearValue: false,
        },
        {
          id: 'cleared',
          name: 'X-Token',
          value: '',
          sensitive: true,
          hasValue: true,
          clearValue: true,
        },
      ]),
    ).toEqual([
      { name: 'Authorization', sensitive: true },
      { name: 'X-Token', sensitive: true, value: null },
    ])
  })

  it('normalizes full server payloads', () => {
    expect(
      buildMCPServerPayload({
        name: ' Linear-Tools ',
        displayName: ' Linear ',
        url: ' https://mcp.example.com/mcp ',
        allowPattern: ' ^linear_ ',
        denyPattern: '',
        enabled: true,
        headers: [
          {
            id: 'plain',
            name: ' X-Workspace ',
            value: ' production ',
            sensitive: false,
            hasValue: true,
            clearValue: false,
          },
        ],
      }),
    ).toEqual({
      name: 'linear-tools',
      displayName: 'Linear',
      url: 'https://mcp.example.com/mcp',
      allowPattern: '^linear_',
      denyPattern: '',
      enabled: true,
      headers: [
        { name: 'X-Workspace', value: 'production', sensitive: false },
      ],
    })
  })
})
