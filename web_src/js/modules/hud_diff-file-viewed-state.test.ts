import {diffTreeStoreSetViewed, reactiveDiffTreeStore} from './diff-file.ts';

// Test 1: Simple Directory - All Files Viewed
test('directory becomes viewed when all files are marked as viewed', () => {
  const store = reactiveDiffTreeStore({
    'TreeRoot': {
      'FullName': '',
      'DisplayName': '',
      'EntryMode': '',
      'IsViewed': false,
      'NameHash': '',
      'DiffStatus': '',
      'Children': [
        {
          'FullName': 'src',
          'DisplayName': 'src',
          'EntryMode': 'tree',
          'IsViewed': false,
          'NameHash': '',
          'DiffStatus': '',
          'Children': [
            {
              'FullName': 'src/file1.js',
              'DisplayName': 'file1.js',
              'EntryMode': '',
              'IsViewed': false,
              'NameHash': 'hash1',
              'DiffStatus': 'added',
              'Children': null,
            },
            {
              'FullName': 'src/file2.js',
              'DisplayName': 'file2.js',
              'EntryMode': '',
              'IsViewed': false,
              'NameHash': 'hash2',
              'DiffStatus': 'modified',
              'Children': null,
            },
            {
              'FullName': 'src/file3.js',
              'DisplayName': 'file3.js',
              'EntryMode': '',
              'IsViewed': false,
              'NameHash': 'hash3',
              'DiffStatus': 'added',
              'Children': null,
            },
          ],
        },
      ],
    },
  });

  // Mark first file - directory should NOT be viewed yet
  diffTreeStoreSetViewed(store, 'src/file1.js', true);
  expect(store.fullNameMap['src'].IsViewed).toBe(false);
  expect(store.fullNameMap['src/file1.js'].IsViewed).toBe(true);

  // Mark second file - directory still NOT viewed
  diffTreeStoreSetViewed(store, 'src/file2.js', true);
  expect(store.fullNameMap['src'].IsViewed).toBe(false);

  // Mark third (last) file - directory SHOULD now be viewed
  diffTreeStoreSetViewed(store, 'src/file3.js', true);
  expect(store.fullNameMap['src'].IsViewed).toBe(true);
  expect(store.fullNameMap['src/file1.js'].IsViewed).toBe(true);
  expect(store.fullNameMap['src/file2.js'].IsViewed).toBe(true);
  expect(store.fullNameMap['src/file3.js'].IsViewed).toBe(true);
});

// Test 2: Simple Directory - Partial Files Viewed
test('directory remains unviewed when only some files are viewed', () => {
  const store = reactiveDiffTreeStore({
    'TreeRoot': {
      'FullName': '',
      'DisplayName': '',
      'EntryMode': '',
      'IsViewed': false,
      'NameHash': '',
      'DiffStatus': '',
      'Children': [
        {
          'FullName': 'docs',
          'DisplayName': 'docs',
          'EntryMode': 'tree',
          'IsViewed': false,
          'NameHash': '',
          'DiffStatus': '',
          'Children': [
            {
              'FullName': 'docs/readme.md',
              'DisplayName': 'readme.md',
              'EntryMode': '',
              'IsViewed': false,
              'NameHash': 'hash1',
              'DiffStatus': 'added',
              'Children': null,
            },
            {
              'FullName': 'docs/guide.md',
              'DisplayName': 'guide.md',
              'EntryMode': '',
              'IsViewed': false,
              'NameHash': 'hash2',
              'DiffStatus': 'added',
              'Children': null,
            },
            {
              'FullName': 'docs/api.md',
              'DisplayName': 'api.md',
              'EntryMode': '',
              'IsViewed': false,
              'NameHash': 'hash3',
              'DiffStatus': 'added',
              'Children': null,
            },
          ],
        },
      ],
    },
  });

  // Mark only 2 out of 3 files as viewed
  diffTreeStoreSetViewed(store, 'docs/readme.md', true);
  diffTreeStoreSetViewed(store, 'docs/guide.md', true);

  // Directory should NOT be viewed because not all children are viewed
  expect(store.fullNameMap['docs'].IsViewed).toBe(false);
  expect(store.fullNameMap['docs/readme.md'].IsViewed).toBe(true);
  expect(store.fullNameMap['docs/guide.md'].IsViewed).toBe(true);
  expect(store.fullNameMap['docs/api.md'].IsViewed).toBe(false);
});

// Test 3: Nested Directory - All Files Viewed Recursively
test('parent and subdirectory both become viewed when all nested files are viewed', () => {
  const store = reactiveDiffTreeStore({
    'TreeRoot': {
      'FullName': '',
      'DisplayName': '',
      'EntryMode': '',
      'IsViewed': false,
      'NameHash': '',
      'DiffStatus': '',
      'Children': [
        {
          'FullName': 'parent',
          'DisplayName': 'parent',
          'EntryMode': 'tree',
          'IsViewed': false,
          'NameHash': '',
          'DiffStatus': '',
          'Children': [
            {
              'FullName': 'parent/file1.txt',
              'DisplayName': 'file1.txt',
              'EntryMode': '',
              'IsViewed': false,
              'NameHash': 'hash1',
              'DiffStatus': 'added',
              'Children': null,
            },
            {
              'FullName': 'parent/subdir',
              'DisplayName': 'subdir',
              'EntryMode': 'tree',
              'IsViewed': false,
              'NameHash': '',
              'DiffStatus': '',
              'Children': [
                {
                  'FullName': 'parent/subdir/file2.txt',
                  'DisplayName': 'file2.txt',
                  'EntryMode': '',
                  'IsViewed': false,
                  'NameHash': 'hash2',
                  'DiffStatus': 'modified',
                  'Children': null,
                },
                {
                  'FullName': 'parent/subdir/file3.txt',
                  'DisplayName': 'file3.txt',
                  'EntryMode': '',
                  'IsViewed': false,
                  'NameHash': 'hash3',
                  'DiffStatus': 'added',
                  'Children': null,
                },
              ],
            },
          ],
        },
      ],
    },
  });

  // Mark files in subdir
  diffTreeStoreSetViewed(store, 'parent/subdir/file2.txt', true);
  expect(store.fullNameMap['parent/subdir'].IsViewed).toBe(false); // Not all children yet
  expect(store.fullNameMap['parent'].IsViewed).toBe(false);

  diffTreeStoreSetViewed(store, 'parent/subdir/file3.txt', true);
  expect(store.fullNameMap['parent/subdir'].IsViewed).toBe(true); // Now subdir is complete
  expect(store.fullNameMap['parent'].IsViewed).toBe(false); // But parent still has file1.txt

  // Mark parent's file
  diffTreeStoreSetViewed(store, 'parent/file1.txt', true);
  expect(store.fullNameMap['parent'].IsViewed).toBe(true); // Now parent is complete too
  expect(store.fullNameMap['parent/subdir'].IsViewed).toBe(true);
});

// Test 4: Nested Directory - Incomplete Subdirectory
test('parent directory remains unviewed when subdirectory is incomplete', () => {
  const store = reactiveDiffTreeStore({
    'TreeRoot': {
      'FullName': '',
      'DisplayName': '',
      'EntryMode': '',
      'IsViewed': false,
      'NameHash': '',
      'DiffStatus': '',
      'Children': [
        {
          'FullName': 'parent',
          'DisplayName': 'parent',
          'EntryMode': 'tree',
          'IsViewed': false,
          'NameHash': '',
          'DiffStatus': '',
          'Children': [
            {
              'FullName': 'parent/file1.txt',
              'DisplayName': 'file1.txt',
              'EntryMode': '',
              'IsViewed': false,
              'NameHash': 'hash1',
              'DiffStatus': 'added',
              'Children': null,
            },
            {
              'FullName': 'parent/subdir',
              'DisplayName': 'subdir',
              'EntryMode': 'tree',
              'IsViewed': false,
              'NameHash': '',
              'DiffStatus': '',
              'Children': [
                {
                  'FullName': 'parent/subdir/file2.txt',
                  'DisplayName': 'file2.txt',
                  'EntryMode': '',
                  'IsViewed': false,
                  'NameHash': 'hash2',
                  'DiffStatus': 'modified',
                  'Children': null,
                },
                {
                  'FullName': 'parent/subdir/file3.txt',
                  'DisplayName': 'file3.txt',
                  'EntryMode': '',
                  'IsViewed': false,
                  'NameHash': 'hash3',
                  'DiffStatus': 'added',
                  'Children': null,
                },
              ],
            },
          ],
        },
      ],
    },
  });

  // Mark parent's direct file and only one file in subdir
  diffTreeStoreSetViewed(store, 'parent/file1.txt', true);
  diffTreeStoreSetViewed(store, 'parent/subdir/file2.txt', true);

  // Subdir is incomplete, so it should not be viewed
  expect(store.fullNameMap['parent/subdir'].IsViewed).toBe(false);
  // Parent cannot be viewed because subdir is not viewed
  expect(store.fullNameMap['parent'].IsViewed).toBe(false);
});

// Test 5: Deep Nesting (3+ Levels)
test('deep nesting - all directories become viewed when all files at deepest level are viewed', () => {
  const store = reactiveDiffTreeStore({
    'TreeRoot': {
      'FullName': '',
      'DisplayName': '',
      'EntryMode': '',
      'IsViewed': false,
      'NameHash': '',
      'DiffStatus': '',
      'Children': [
        {
          'FullName': 'level1',
          'DisplayName': 'level1',
          'EntryMode': 'tree',
          'IsViewed': false,
          'NameHash': '',
          'DiffStatus': '',
          'Children': [
            {
              'FullName': 'level1/level2',
              'DisplayName': 'level2',
              'EntryMode': 'tree',
              'IsViewed': false,
              'NameHash': '',
              'DiffStatus': '',
              'Children': [
                {
                  'FullName': 'level1/level2/level3',
                  'DisplayName': 'level3',
                  'EntryMode': 'tree',
                  'IsViewed': false,
                  'NameHash': '',
                  'DiffStatus': '',
                  'Children': [
                    {
                      'FullName': 'level1/level2/level3/file1.txt',
                      'DisplayName': 'file1.txt',
                      'EntryMode': '',
                      'IsViewed': false,
                      'NameHash': 'hash1',
                      'DiffStatus': 'added',
                      'Children': null,
                    },
                    {
                      'FullName': 'level1/level2/level3/file2.txt',
                      'DisplayName': 'file2.txt',
                      'EntryMode': '',
                      'IsViewed': false,
                      'NameHash': 'hash2',
                      'DiffStatus': 'modified',
                      'Children': null,
                    },
                  ],
                },
              ],
            },
          ],
        },
      ],
    },
  });

  // Mark first file
  diffTreeStoreSetViewed(store, 'level1/level2/level3/file1.txt', true);
  expect(store.fullNameMap['level1/level2/level3'].IsViewed).toBe(false);
  expect(store.fullNameMap['level1/level2'].IsViewed).toBe(false);
  expect(store.fullNameMap['level1'].IsViewed).toBe(false);

  // Mark second file - all levels should cascade to viewed
  diffTreeStoreSetViewed(store, 'level1/level2/level3/file2.txt', true);
  expect(store.fullNameMap['level1/level2/level3'].IsViewed).toBe(true);
  expect(store.fullNameMap['level1/level2'].IsViewed).toBe(true);
  expect(store.fullNameMap['level1'].IsViewed).toBe(true);
});

// Test 6: Unmarking Propagates Upward
test('unmarking a file causes parent directories to become unviewed', () => {
  const store = reactiveDiffTreeStore({
    'TreeRoot': {
      'FullName': '',
      'DisplayName': '',
      'EntryMode': '',
      'IsViewed': false,
      'NameHash': '',
      'DiffStatus': '',
      'Children': [
        {
          'FullName': 'parent',
          'DisplayName': 'parent',
          'EntryMode': 'tree',
          'IsViewed': false,
          'NameHash': '',
          'DiffStatus': '',
          'Children': [
            {
              'FullName': 'parent/file1.txt',
              'DisplayName': 'file1.txt',
              'EntryMode': '',
              'IsViewed': false,
              'NameHash': 'hash1',
              'DiffStatus': 'added',
              'Children': null,
            },
            {
              'FullName': 'parent/subdir',
              'DisplayName': 'subdir',
              'EntryMode': 'tree',
              'IsViewed': false,
              'NameHash': '',
              'DiffStatus': '',
              'Children': [
                {
                  'FullName': 'parent/subdir/file2.txt',
                  'DisplayName': 'file2.txt',
                  'EntryMode': '',
                  'IsViewed': false,
                  'NameHash': 'hash2',
                  'DiffStatus': 'modified',
                  'Children': null,
                },
                {
                  'FullName': 'parent/subdir/file3.txt',
                  'DisplayName': 'file3.txt',
                  'EntryMode': '',
                  'IsViewed': false,
                  'NameHash': 'hash3',
                  'DiffStatus': 'added',
                  'Children': null,
                },
              ],
            },
          ],
        },
      ],
    },
  });

  // Mark all files as viewed
  diffTreeStoreSetViewed(store, 'parent/file1.txt', true);
  diffTreeStoreSetViewed(store, 'parent/subdir/file2.txt', true);
  diffTreeStoreSetViewed(store, 'parent/subdir/file3.txt', true);

  // Verify all are viewed
  expect(store.fullNameMap['parent/subdir'].IsViewed).toBe(true);
  expect(store.fullNameMap['parent'].IsViewed).toBe(true);

  // Unmark one file in subdir
  diffTreeStoreSetViewed(store, 'parent/subdir/file3.txt', false);

  // Subdir should become unviewed
  expect(store.fullNameMap['parent/subdir'].IsViewed).toBe(false);
  // Parent should also become unviewed (because subdir is no longer viewed)
  expect(store.fullNameMap['parent'].IsViewed).toBe(false);
  // The other files should still be viewed
  expect(store.fullNameMap['parent/file1.txt'].IsViewed).toBe(true);
  expect(store.fullNameMap['parent/subdir/file2.txt'].IsViewed).toBe(true);
});

// Test 7: Complex Mixed Structure
test('complex structure - partial completion of directories', () => {
  const store = reactiveDiffTreeStore({
    'TreeRoot': {
      'FullName': '',
      'DisplayName': '',
      'EntryMode': '',
      'IsViewed': false,
      'NameHash': '',
      'DiffStatus': '',
      'Children': [
        {
          'FullName': 'root',
          'DisplayName': 'root',
          'EntryMode': 'tree',
          'IsViewed': false,
          'NameHash': '',
          'DiffStatus': '',
          'Children': [
            {
              'FullName': 'root/dirA',
              'DisplayName': 'dirA',
              'EntryMode': 'tree',
              'IsViewed': false,
              'NameHash': '',
              'DiffStatus': '',
              'Children': [
                {
                  'FullName': 'root/dirA/file1.txt',
                  'DisplayName': 'file1.txt',
                  'EntryMode': '',
                  'IsViewed': false,
                  'NameHash': 'hash1',
                  'DiffStatus': 'added',
                  'Children': null,
                },
                {
                  'FullName': 'root/dirA/file2.txt',
                  'DisplayName': 'file2.txt',
                  'EntryMode': '',
                  'IsViewed': false,
                  'NameHash': 'hash2',
                  'DiffStatus': 'modified',
                  'Children': null,
                },
              ],
            },
            {
              'FullName': 'root/dirB',
              'DisplayName': 'dirB',
              'EntryMode': 'tree',
              'IsViewed': false,
              'NameHash': '',
              'DiffStatus': '',
              'Children': [
                {
                  'FullName': 'root/dirB/file3.txt',
                  'DisplayName': 'file3.txt',
                  'EntryMode': '',
                  'IsViewed': false,
                  'NameHash': 'hash3',
                  'DiffStatus': 'added',
                  'Children': null,
                },
                {
                  'FullName': 'root/dirB/subB',
                  'DisplayName': 'subB',
                  'EntryMode': 'tree',
                  'IsViewed': false,
                  'NameHash': '',
                  'DiffStatus': '',
                  'Children': [
                    {
                      'FullName': 'root/dirB/subB/file4.txt',
                      'DisplayName': 'file4.txt',
                      'EntryMode': '',
                      'IsViewed': false,
                      'NameHash': 'hash4',
                      'DiffStatus': 'deleted',
                      'Children': null,
                    },
                  ],
                },
              ],
            },
            {
              'FullName': 'root/file5.txt',
              'DisplayName': 'file5.txt',
              'EntryMode': '',
              'IsViewed': false,
              'NameHash': 'hash5',
              'DiffStatus': 'renamed',
              'Children': null,
            },
          ],
        },
      ],
    },
  });

  // Mark all files in dirA as viewed
  diffTreeStoreSetViewed(store, 'root/dirA/file1.txt', true);
  diffTreeStoreSetViewed(store, 'root/dirA/file2.txt', true);

  // dirA should be viewed
  expect(store.fullNameMap['root/dirA'].IsViewed).toBe(true);
  // But root should NOT be viewed (dirB and file5.txt not complete)
  expect(store.fullNameMap['root'].IsViewed).toBe(false);

  // Mark only some files in dirB
  diffTreeStoreSetViewed(store, 'root/dirB/file3.txt', true);
  // dirB should NOT be viewed (subB is not complete)
  expect(store.fullNameMap['root/dirB'].IsViewed).toBe(false);
  expect(store.fullNameMap['root/dirB/subB'].IsViewed).toBe(false);

  // Now complete dirB by marking subB's file
  diffTreeStoreSetViewed(store, 'root/dirB/subB/file4.txt', true);
  expect(store.fullNameMap['root/dirB/subB'].IsViewed).toBe(true);
  expect(store.fullNameMap['root/dirB'].IsViewed).toBe(true);
  // But root still not viewed (file5.txt remains)
  expect(store.fullNameMap['root'].IsViewed).toBe(false);

  // Complete root by marking file5
  diffTreeStoreSetViewed(store, 'root/file5.txt', true);
  expect(store.fullNameMap['root'].IsViewed).toBe(true);
});

// Test 8: Initially Pre-Viewed Files
test('directory state is correctly computed on initialization with pre-viewed files', () => {
  const store = reactiveDiffTreeStore({
    'TreeRoot': {
      'FullName': '',
      'DisplayName': '',
      'EntryMode': '',
      'IsViewed': false,
      'NameHash': '',
      'DiffStatus': '',
      'Children': [
        {
          'FullName': 'complete-dir',
          'DisplayName': 'complete-dir',
          'EntryMode': 'tree',
          'IsViewed': false,
          'NameHash': '',
          'DiffStatus': '',
          'Children': [
            {
              'FullName': 'complete-dir/file1.txt',
              'DisplayName': 'file1.txt',
              'EntryMode': '',
              'IsViewed': true, // Pre-viewed
              'NameHash': 'hash1',
              'DiffStatus': 'added',
              'Children': null,
            },
            {
              'FullName': 'complete-dir/file2.txt',
              'DisplayName': 'file2.txt',
              'EntryMode': '',
              'IsViewed': true, // Pre-viewed
              'NameHash': 'hash2',
              'DiffStatus': 'modified',
              'Children': null,
            },
          ],
        },
        {
          'FullName': 'partial-dir',
          'DisplayName': 'partial-dir',
          'EntryMode': 'tree',
          'IsViewed': false,
          'NameHash': '',
          'DiffStatus': '',
          'Children': [
            {
              'FullName': 'partial-dir/file3.txt',
              'DisplayName': 'file3.txt',
              'EntryMode': '',
              'IsViewed': true, // Pre-viewed
              'NameHash': 'hash3',
              'DiffStatus': 'added',
              'Children': null,
            },
            {
              'FullName': 'partial-dir/file4.txt',
              'DisplayName': 'file4.txt',
              'EntryMode': '',
              'IsViewed': false, // Not viewed
              'NameHash': 'hash4',
              'DiffStatus': 'deleted',
              'Children': null,
            },
          ],
        },
      ],
    },
  });

  // complete-dir should be automatically viewed on initialization
  expect(store.fullNameMap['complete-dir'].IsViewed).toBe(true);
  // partial-dir should NOT be viewed
  expect(store.fullNameMap['partial-dir'].IsViewed).toBe(false);
});

// Test 9: Multi-level Partial Completion at Deepest Level
test('incomplete state at deepest level prevents all ancestor directories from being viewed', () => {
  const store = reactiveDiffTreeStore({
    'TreeRoot': {
      'FullName': '',
      'DisplayName': '',
      'EntryMode': '',
      'IsViewed': false,
      'NameHash': '',
      'DiffStatus': '',
      'Children': [
        {
          'FullName': 'a',
          'DisplayName': 'a',
          'EntryMode': 'tree',
          'IsViewed': false,
          'NameHash': '',
          'DiffStatus': '',
          'Children': [
            {
              'FullName': 'a/b',
              'DisplayName': 'b',
              'EntryMode': 'tree',
              'IsViewed': false,
              'NameHash': '',
              'DiffStatus': '',
              'Children': [
                {
                  'FullName': 'a/b/c',
                  'DisplayName': 'c',
                  'EntryMode': 'tree',
                  'IsViewed': false,
                  'NameHash': '',
                  'DiffStatus': '',
                  'Children': [
                    {
                      'FullName': 'a/b/c/file1.txt',
                      'DisplayName': 'file1.txt',
                      'EntryMode': '',
                      'IsViewed': false,
                      'NameHash': 'hash1',
                      'DiffStatus': 'added',
                      'Children': null,
                    },
                    {
                      'FullName': 'a/b/c/file2.txt',
                      'DisplayName': 'file2.txt',
                      'EntryMode': '',
                      'IsViewed': false,
                      'NameHash': 'hash2',
                      'DiffStatus': 'modified',
                      'Children': null,
                    },
                  ],
                },
              ],
            },
          ],
        },
      ],
    },
  });

  // Mark only one file at the deepest level
  diffTreeStoreSetViewed(store, 'a/b/c/file1.txt', true);

  // None of the directories should be viewed
  expect(store.fullNameMap['a/b/c'].IsViewed).toBe(false);
  expect(store.fullNameMap['a/b'].IsViewed).toBe(false);
  expect(store.fullNameMap['a'].IsViewed).toBe(false);

  // Only the marked file should be viewed
  expect(store.fullNameMap['a/b/c/file1.txt'].IsViewed).toBe(true);
  expect(store.fullNameMap['a/b/c/file2.txt'].IsViewed).toBe(false);

  // Now mark the second file
  diffTreeStoreSetViewed(store, 'a/b/c/file2.txt', true);

  // All directories should now cascade to viewed
  expect(store.fullNameMap['a/b/c'].IsViewed).toBe(true);
  expect(store.fullNameMap['a/b'].IsViewed).toBe(true);
  expect(store.fullNameMap['a'].IsViewed).toBe(true);
});

