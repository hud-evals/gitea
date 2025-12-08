// web_src/js/modules/diff-file.ts
import {reactive} from 'vue';

export function reactiveDiffTreeStore(data) {
  const store = reactive({fullNameMap: {}});
  buildMap(store.fullNameMap, data.TreeRoot);
  return store;
}

// TODO: Implement recursive directory viewed state propagation
export function diffTreeStoreSetViewed(store, fullName, viewed) {
  const entry = store.fullNameMap[fullName];
  if (entry) entry.IsViewed = viewed;
}

function buildMap(map, entry) {
  if (!entry) return;
  map[entry.FullName] = entry;
  if (entry.Children) {
    entry.Children.forEach(child => buildMap(map, child));
  }
}