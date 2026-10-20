// Stands in for elkjs, which mermaid 12 depends on for its ELK layouts.
//
// elkjs is EPL-2.0, and Notrios ships only MIT/Apache-2.0 compatible code, so
// package.json overrides mermaid's elkjs with this package and nothing EPL is
// downloaded, installed or bundled (v1.0 J45). mermaid 11 shipped no ELK at
// all, and src/mermaid-render.ts lays diagrams out with dagre, so this keeps
// what renders where it was: a diagram asking for an ELK layout fails the way
// any diagram that will not render does, and the preview keeps its source.

export const ELK_REFUSAL = 'ELK layouts are not included in Notrios';

export default class ELK {
  layout() {
    return Promise.reject(new Error(ELK_REFUSAL));
  }
}
