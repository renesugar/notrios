import { sourceForTypeScriptAnchor } from './docaudit_ts.mjs';

function parseArguments(argv) {
  if (!Array.isArray(argv) || argv.length < 4 || argv[0] !== '--root') {
    throw new Error('usage: doccheck_ts.mjs --root <repo-root> --anchor ts:<module>#<symbol> [--anchor ...]');
  }
  const root = argv[1];
  const anchors = [];
  for (let index = 2; index < argv.length; index += 2) {
    if (argv[index] !== '--anchor' || !argv[index + 1]) {
      throw new Error('arguments after --root must be --anchor <ts:<module>#<symbol>> pairs');
    }
    anchors.push(argv[index + 1]);
  }
  return { root, anchors };
}

try {
  const { root, anchors } = parseArguments(process.argv.slice(2));
  const focusSymbols = anchors.slice(1).map((anchor) => anchor.slice(anchor.lastIndexOf('#') + 1));
  const slices = anchors.map((anchor, index) => {
    const result = sourceForTypeScriptAnchor(root, anchor);
    if (index !== 0 || result.source.length <= 24 * 1024 || focusSymbols.length === 0) return result;
    const headerEnd = result.source.indexOf('{');
    const excerpts = [];
    for (const symbol of focusSymbols) {
      let offset = result.source.indexOf(symbol);
      let count = 0;
      while (offset >= 0 && count < 4) {
        const start = Math.max(0, result.source.lastIndexOf('\n', Math.max(0, offset - 320)) + 1);
        let end = result.source.indexOf('\n', offset + symbol.length + 480);
        if (end < 0) end = Math.min(result.source.length, offset + symbol.length + 480);
        excerpts.push(`FOCUSED CALL SITE ${symbol}\n${result.source.slice(start, end)}`);
        offset = result.source.indexOf(symbol, end);
        count += 1;
      }
    }
    if (excerpts.length === 0) throw new Error(`oversized root ${anchor} has no focused direct-callee call sites`);
    const header = headerEnd >= 0 ? result.source.slice(0, headerEnd + 1) : result.source.slice(0, 512);
    return {
      anchor: result.anchor,
      source: `${header}\n/* Unrelated root body omitted by the recorded bounded decomposition. */\n${excerpts.join('\n\n')}\n}`,
    };
  });
  process.stdout.write(`${JSON.stringify({ slices })}\n`);
} catch (error) {
  const message = error instanceof Error ? error.message : String(error);
  process.stderr.write(`doccheck_ts: ${message}\n`);
  process.exitCode = 1;
}
