// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

import {generateElementId} from './dom.ts';

test('generateElementId creates unique IDs', () => {
  // Test that generated IDs are unique
  const id1 = generateElementId();
  const id2 = generateElementId();
  const id3 = generateElementId();

  expect(id1).not.toBe(id2);
  expect(id2).not.toBe(id3);
  expect(id1).not.toBe(id3);
});

test('generateElementId with prefix', () => {
  // Test ID generation with prefix
  const prefix = 'test-prefix';
  const id = generateElementId(prefix);

  expect(id).toContain(prefix);
  expect(id).toMatch(/^test-prefix-\d+$/);
});

test('generateElementId increments correctly', () => {
  // Test that IDs increment sequentially
  const ids = [];
  for (let i = 0; i < 10; i++) {
    ids.push(generateElementId());
  }

  // All should be unique
  const uniqueIds = new Set(ids);
  expect(uniqueIds.size).toBe(10);
});

test('generateElementId without prefix', () => {
  // Test ID generation without prefix
  const id = generateElementId();

  expect(id).toBeTruthy();
  expect(id).toMatch(/^\d+$/);
});

test('generateElementId consistency', () => {
  // Test that function behavior is consistent
  const id1 = generateElementId('prefix');
  const id2 = generateElementId('prefix');

  // Should both start with prefix
  expect(id1).toContain('prefix');
  expect(id2).toContain('prefix');

  // But be different
  expect(id1).not.toBe(id2);
});

test('generateElementId handles empty string prefix', () => {
  // Test with empty string prefix
  const id = generateElementId('');

  expect(id).toBeTruthy();
  // Should still generate valid ID
  expect(id.length).toBeGreaterThan(0);
});

test('generateElementId handles special characters in prefix', () => {
  // Test with special characters
  const prefixes = ['test-123', 'test_abc', 'test.xyz'];

  for (const prefix of prefixes) {
    const id = generateElementId(prefix);
    expect(id).toContain(prefix);
    expect(id).toBeTruthy();
  }
});

test('generateElementId maintains global counter', () => {
  // Test that there's a global counter being incremented
  const id1 = generateElementId();
  const id2 = generateElementId();

  // Extract numbers and verify they're sequential
  const num1 = parseInt(id1);
  const num2 = parseInt(id2);

  expect(num2).toBe(num1 + 1);
});

test('generateElementId thread safety simulation', () => {
  // Simulate rapid ID generation
  const ids = new Set();

  for (let i = 0; i < 1000; i++) {
    const id = generateElementId();
    ids.add(id);
  }

  // All 1000 should be unique
  expect(ids.size).toBe(1000);
});

test('generateElementId prefix variations', () => {
  // Test different prefix patterns
  const testCases = [
    {prefix: 'modal', expected: /^modal-\d+$/},
    {prefix: 'dropdown', expected: /^dropdown-\d+$/},
    {prefix: 'form', expected: /^form-\d+$/},
    {prefix: 'element', expected: /^element-\d+$/},
  ];

  for (const testCase of testCases) {
    const id = generateElementId(testCase.prefix);
    expect(id).toMatch(testCase.expected);
  }
});

test('generateElementId numeric prefix', () => {
  // Test with numeric prefix
  const id = generateElementId('123');

  expect(id).toContain('123');
  expect(id).toBeTruthy();
});

test('generateElementId long prefix', () => {
  // Test with very long prefix
  const longPrefix = 'a'.repeat(100);
  const id = generateElementId(longPrefix);

  expect(id).toContain(longPrefix);
  expect(id).toBeTruthy();
});

test('generateElementId maintains uniqueness across prefixes', () => {
  // IDs with different prefixes should still be unique globally
  const id1 = generateElementId('prefix1');
  const id2 = generateElementId('prefix2');
  const id3 = generateElementId('prefix1');

  // All should be different
  expect(id1).not.toBe(id2);
  expect(id2).not.toBe(id3);
  expect(id1).not.toBe(id3);
});

test('generateElementId can be used in DOM', () => {
  // Test that generated IDs are valid for DOM usage
  const id = generateElementId('test');

  // Should be valid DOM ID (no spaces, starts with letter or underscore)
  expect(id).toMatch(/^[a-zA-Z_][\w-]*$/);
});

test('generateElementId performance', () => {
  // Test that ID generation is fast
  const start = Date.now();

  for (let i = 0; i < 10000; i++) {
    generateElementId();
  }

  const duration = Date.now() - start;

  // Should complete quickly (< 100ms for 10k IDs)
  expect(duration).toBeLessThan(100);
});
