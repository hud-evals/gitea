// web_src/js/modules/diff-file.ts
// TODO: Add recursive directory viewed state propagation
import {reactive} from 'vue';

export function reactiveDiffTreeStore(data) {
  const store = reactive({
    diffFileTree: data,
    fileTreeIsVisible: false,
    selectedItem: '',
    fullNameMap: {},
  });
  buildFullNameMap(store.fullNameMap, data.TreeRoot);
  return store;
}

/**
 * Sets the viewed state for a file
 */
export function diffTreeStoreSetViewed(store, fullName, viewed) {
  const entry = store.fullNameMap[fullName];
  if (!entry) return;
  entry.IsViewed = viewed;
  // TODO: Propagate viewed state to parent directories
}

function buildFullNameMap(map, entry) {
  if (!entry) return;
  map[entry.FullName] = entry;
  if (entry.Children) {
    for (const child of entry.Children) {
      child.ParentEntry = entry;
      buildFullNameMap(map, child);
    }
  }
}