import { existsSync, realpathSync, readFileSync, statSync } from 'node:fs';
import { createRequire } from 'node:module';
import { basename, dirname, extname, isAbsolute, relative, resolve, sep } from 'node:path';
import { pathToFileURL } from 'node:url';

// Keep the compiler dependency owned by the web workspace.  Loading it from
// this package file also makes importing the resolver independent of cwd.
function webPackagePath() {
  // Vite rewrites import.meta.url while running Vitest, so locate the package
  // from cwd (the normal npm workspace) and, for the CLI, from its script.
  const candidates = [];
  if (process.argv[1] && basename(process.argv[1]) === 'docaudit_ts.mjs') {
    const scriptRoot = resolve(dirname(process.argv[1]), '..');
    candidates.push(resolve(scriptRoot, 'web/package.json'));
  }
  candidates.push(
    resolve(process.cwd(), 'web/package.json'),
    resolve(process.cwd(), 'package.json'),
  );
  const packagePath = candidates.find((candidate) => existsSync(candidate));
  if (!packagePath) throw new Error('cannot locate the web workspace package.json');
  return packagePath;
}

const requireFromWeb = createRequire(webPackagePath());
const compiler = requireFromWeb('typescript');
// TypeScript 7 exposes its AST/compiler pieces through the unstable API while
// older releases expose the same API at the package root.  Both are the
// installed compiler from web/package.json; the fallback keeps this resolver
// usable across the repository's supported npm lockfile revisions.
const ts = compiler.SyntaxKind && compiler.createScanner
  ? compiler
  : await import(pathToFileURL(requireFromWeb.resolve('typescript/unstable/ast')));

const ANCHOR_PREFIX = 'ts:';
const SYMBOL_RE = /^[A-Za-z_$][A-Za-z0-9_$]*$/u;
const MODULE_EXTENSIONS = new Set(['.ts', '.tsx']);
const DECLARATION_KINDS = new Map([
  [ts.SyntaxKind.FunctionDeclaration, 'function'],
  [ts.SyntaxKind.ClassDeclaration, 'class'],
  [ts.SyntaxKind.InterfaceDeclaration, 'interface'],
  [ts.SyntaxKind.TypeAliasDeclaration, 'type'],
  [ts.SyntaxKind.EnumDeclaration, 'enum'],
]);

export class DocAuditTypeScriptError extends Error {
  constructor(message) {
    super(message);
    this.name = 'DocAuditTypeScriptError';
  }
}

function fail(message) {
  throw new DocAuditTypeScriptError(message);
}

/**
 * Parse the frozen TS anchor form without touching the filesystem.
 *
 * @param {string} anchor
 * @returns {{ anchor: string, module: string, symbol: string }}
 */
export function parseTypeScriptAnchor(anchor) {
  if (typeof anchor !== 'string' || !anchor.startsWith(ANCHOR_PREFIX)) {
    fail(`malformed TypeScript source-symbol anchor: ${String(anchor)}`);
  }
  const body = anchor.slice(ANCHOR_PREFIX.length);
  const separator = body.indexOf('#');
  if (separator <= 0 || separator !== body.lastIndexOf('#')) {
    fail(`malformed TypeScript source-symbol anchor: ${anchor}`);
  }
  const module = body.slice(0, separator);
  const symbol = body.slice(separator + 1);
  if (!module || !symbol || !SYMBOL_RE.test(symbol)) {
    fail(`malformed TypeScript source-symbol anchor: ${anchor}`);
  }
  return { anchor, module, symbol };
}

function repositoryFile(root, module, anchor) {
  if (module.includes('\0') || module.includes('\\') || isAbsolute(module)) {
    fail(`repository-escaping TypeScript source-symbol anchor: ${anchor}`);
  }
  const parts = module.split('/');
  if (parts.some((part) => part === '..' || part === '')) {
    fail(`repository-escaping TypeScript source-symbol anchor: ${anchor}`);
  }
  if (!MODULE_EXTENSIONS.has(extname(module))) {
    fail(`unsupported TypeScript module in source-symbol anchor: ${anchor}`);
  }

  let rootPath;
  try {
    rootPath = realpathSync(resolve(root));
  } catch {
    fail(`repository root does not exist: ${root}`);
  }
  const candidate = resolve(rootPath, ...parts);
  let modulePath;
  try {
    modulePath = realpathSync(candidate);
    if (!statSync(modulePath).isFile()) {
      fail(`TypeScript module is not a file: ${anchor}`);
    }
  } catch {
    fail(`missing TypeScript module in source-symbol anchor: ${anchor}`);
  }
  const outside = relative(rootPath, modulePath);
  if (!outside || isAbsolute(outside) || outside === '..' || outside.startsWith(`..${sep}`)) {
    fail(`repository-escaping TypeScript source-symbol anchor: ${anchor}`);
  }
  return { rootPath, modulePath, module: outside.split(sep).join('/') };
}

function namedTopLevelDeclarations(sourceFile, symbol) {
  const matches = [];
  for (const statement of sourceFile.statements) {
    const declarationKind = DECLARATION_KINDS.get(statement.kind);
    if (declarationKind && statement.name && ts.isIdentifier(statement.name)
      && statement.name.text === symbol) {
      matches.push({ node: statement, kind: declarationKind });
      continue;
    }
    if (ts.isVariableStatement(statement)) {
      for (const declaration of statement.declarationList.declarations) {
        if (ts.isIdentifier(declaration.name) && declaration.name.text === symbol) {
          matches.push({ node: declaration, kind: 'variable' });
        }
      }
    }
  }
  return matches;
}

/**
 * Resolve one TS/TSX source-symbol anchor.
 *
 * @param {string} root repository root
 * @param {string} anchor frozen ts:<module>#<symbol> anchor
 * @returns {{ anchor: string, kind: string, module: string, symbol: string }}
 */
export function resolveTypeScriptAnchor(root, anchor) {
  const parsed = parseTypeScriptAnchor(anchor);
  const located = repositoryFile(root, parsed.module, anchor);
  const source = readFileSync(located.modulePath, 'utf8');
  const scriptKind = extname(located.modulePath) === '.tsx' ? ts.ScriptKind.TSX : ts.ScriptKind.TS;
  const matches = resolveTypeScriptSource(source, parsed.symbol, scriptKind);
  if (matches.length === 0) {
    fail(`dangling TypeScript source-symbol anchor: ${anchor}`);
  }
  if (matches.length !== 1) {
    fail(`ambiguous TypeScript source-symbol anchor: ${anchor}`);
  }
  return {
    anchor,
    kind: matches[0].kind,
    module: located.module,
    symbol: parsed.symbol,
  };
}

function scanTopLevelDeclarations(source, symbol, scriptKind) {
  const scanner = ts.createScanner(
    true,
    scriptKind === ts.ScriptKind.TSX ? ts.LanguageVariant.JSX : ts.LanguageVariant.Standard,
    source,
  );
  const matches = [];
  let braceDepth = 0;
  let parenDepth = 0;
  let bracketDepth = 0;
  let statementStart = true;
  let variableList = false;
  const templateExpressionDepths = [];
  let token = scanner.scan();
  while (token !== ts.SyntaxKind.EndOfFile) {
    const atTopLevel = braceDepth === 0 && parenDepth === 0 && bracketDepth === 0;
    if (token === ts.SyntaxKind.TemplateHead || token === ts.SyntaxKind.TemplateMiddle) {
      templateExpressionDepths.push(1);
    }
    if (token === ts.SyntaxKind.CloseBraceToken && templateExpressionDepths.length > 0) {
      const currentDepth = templateExpressionDepths.length - 1;
      templateExpressionDepths[currentDepth] -= 1;
      if (templateExpressionDepths[currentDepth] === 0) {
        templateExpressionDepths.pop();
        token = scanner.reScanTemplateToken(false);
        if (token === ts.SyntaxKind.TemplateMiddle) templateExpressionDepths.push(1);
        continue;
      }
    }
    if (atTopLevel && (statementStart || scanner.hasPrecedingLineBreak())) {
      const declaration = new Map([
        [ts.SyntaxKind.FunctionKeyword, 'function'],
        [ts.SyntaxKind.ClassKeyword, 'class'],
        [ts.SyntaxKind.InterfaceKeyword, 'interface'],
        [ts.SyntaxKind.TypeKeyword, 'type'],
        [ts.SyntaxKind.EnumKeyword, 'enum'],
      ]).get(token);
      const modifier = token === ts.SyntaxKind.ExportKeyword
        || token === ts.SyntaxKind.DefaultKeyword
        || token === ts.SyntaxKind.DeclareKeyword
        || token === ts.SyntaxKind.AbstractKeyword
        || token === ts.SyntaxKind.AsyncKeyword;
      if (declaration) {
        variableList = false;
        const nameToken = scanner.scan();
        if (nameToken === ts.SyntaxKind.Identifier && scanner.getTokenValue() === symbol) {
          matches.push({ kind: declaration });
        }
        token = nameToken;
        statementStart = false;
        continue;
      }
      if (token === ts.SyntaxKind.ConstKeyword || token === ts.SyntaxKind.LetKeyword
        || token === ts.SyntaxKind.VarKeyword) {
        variableList = true;
        const nameToken = scanner.scan();
        if (nameToken === ts.SyntaxKind.Identifier && scanner.getTokenValue() === symbol) {
          matches.push({ kind: 'variable' });
        }
        token = nameToken;
        statementStart = false;
        continue;
      }
      if (modifier) {
        token = scanner.scan();
        continue;
      }
    }
    if (token === ts.SyntaxKind.OpenBraceToken) {
      braceDepth += 1;
      if (templateExpressionDepths.length > 0) {
        templateExpressionDepths[templateExpressionDepths.length - 1] += 1;
      }
    }
    else if (token === ts.SyntaxKind.CloseBraceToken) {
      braceDepth = Math.max(0, braceDepth - 1);
      if (braceDepth === 0 && parenDepth === 0 && bracketDepth === 0) statementStart = true;
    } else if (token === ts.SyntaxKind.OpenParenToken) parenDepth += 1;
    else if (token === ts.SyntaxKind.CloseParenToken) parenDepth = Math.max(0, parenDepth - 1);
    else if (token === ts.SyntaxKind.OpenBracketToken) bracketDepth += 1;
    else if (token === ts.SyntaxKind.CloseBracketToken) bracketDepth = Math.max(0, bracketDepth - 1);
    if (atTopLevel && variableList && token === ts.SyntaxKind.CommaToken) {
      const nameToken = scanner.scan();
      if (nameToken === ts.SyntaxKind.Identifier && scanner.getTokenValue() === symbol) {
        matches.push({ kind: 'variable' });
      }
      token = nameToken;
      continue;
    }
    if (atTopLevel && token === ts.SyntaxKind.SemicolonToken) {
      statementStart = true;
      variableList = false;
    }
    else if (atTopLevel && statementStart && token !== ts.SyntaxKind.EndOfFile) statementStart = false;
    token = scanner.scan();
  }
  return matches;
}

function resolveTypeScriptSource(source, symbol, scriptKind) {
  if (typeof ts.createSourceFile === 'function') {
    const sourceFile = ts.createSourceFile(
      '<docaudit>',
      source,
      ts.ScriptTarget.Latest,
      true,
      scriptKind,
    );
    return namedTopLevelDeclarations(sourceFile, symbol);
  }
  return scanTopLevelDeclarations(source, symbol, scriptKind);
}

/** Resolve a declaration from source text; useful for deterministic fixtures. */
export function resolveTypeScriptSourceText(source, symbol, module = '<fixture>.ts') {
  if (typeof source !== 'string' || !SYMBOL_RE.test(symbol)) {
    fail(`malformed TypeScript source-symbol fixture: ${String(symbol)}`);
  }
  const scriptKind = module.endsWith('.tsx') ? ts.ScriptKind.TSX : ts.ScriptKind.TS;
  const matches = resolveTypeScriptSource(source, symbol, scriptKind);
  if (matches.length === 0) fail(`dangling TypeScript source-symbol anchor: ${symbol}`);
  if (matches.length !== 1) fail(`ambiguous TypeScript source-symbol anchor: ${symbol}`);
  return { kind: matches[0].kind, module, symbol };
}

/**
 * Resolve anchors in lexical anchor order, making CLI and library output
 * stable regardless of argument order.
 *
 * @param {string} root repository root
 * @param {readonly string[]} anchors
 * @returns {Array<{ anchor: string, kind: string, module: string, symbol: string }>}
 */
export function resolveTypeScriptAnchors(root, anchors) {
  if (!Array.isArray(anchors) || anchors.length === 0) {
    fail('at least one TypeScript source-symbol anchor is required');
  }
  const resolved = anchors.map((anchor) => resolveTypeScriptAnchor(root, anchor));
  resolved.sort((left, right) => left.anchor < right.anchor ? -1 : left.anchor > right.anchor ? 1 : 0);
  return resolved;
}

/**
 * Parse the intentionally small command-line interface.
 *
 * @param {readonly string[]} argv
 * @returns {{ root: string, anchors: string[] }}
 */
export function parseArguments(argv) {
  if (!Array.isArray(argv) || argv.length < 4 || argv[0] !== '--root') {
    fail('usage: docaudit_ts.mjs --root <repo-root> --anchor ts:<module>#<symbol> [--anchor ...]');
  }
  const root = argv[1];
  if (!root || root.startsWith('--')) {
    fail('missing repository root after --root');
  }
  const anchors = [];
  for (let index = 2; index < argv.length; index += 2) {
    if (argv[index] !== '--anchor' || !argv[index + 1] || argv[index + 1].startsWith('--')) {
      fail('arguments after --root must be --anchor <ts:<module>#<symbol>> pairs');
    }
    anchors.push(argv[index + 1]);
  }
  if (anchors.length === 0) {
    fail('at least one --anchor is required');
  }
  return { root, anchors };
}

export function run(argv = process.argv.slice(2)) {
  const options = parseArguments(argv);
  return resolveTypeScriptAnchors(options.root, options.anchors);
}

function isMainModule() {
  if (!process.argv[1]) return false;
  return import.meta.url === pathToFileURL(resolve(process.argv[1])).href;
}

if (isMainModule()) {
  try {
    process.stdout.write(`${JSON.stringify(run())}\n`);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    process.stderr.write(`docaudit_ts: ${message}\n`);
    process.exitCode = 1;
  }
}
