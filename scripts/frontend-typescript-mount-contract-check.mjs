#!/usr/bin/env node

import { readFile } from 'node:fs/promises';
import { resolve } from 'node:path';

import ts from '../frontend/node_modules/typescript/lib/typescript.js';

function fail(message) {
  process.stderr.write(`frontend mount contract check: ${message}\n`);
  process.exitCode = 1;
}

function parseArguments(argv) {
  const values = new Map();
  for (let index = 0; index < argv.length; index += 2) {
    const flag = argv[index];
    const value = argv[index + 1];
    if (!flag?.startsWith('--') || value === undefined || value.startsWith('--')) {
      throw new Error('arguments must be exact --name value pairs');
    }
    if (values.has(flag)) throw new Error(`duplicate argument: ${flag}`);
    values.set(flag, value);
  }
  const required = ['--kind', '--source', '--function', '--root', '--root-type', '--parameter', '--parameter-type'];
  for (const flag of required) {
    if (!values.has(flag)) throw new Error(`missing required argument: ${flag}`);
  }
  const allowed = new Set([...required, '--require-interface', '--exclusive-callable-type-user']);
  for (const flag of values.keys()) {
    if (!allowed.has(flag)) throw new Error(`unknown argument: ${flag}`);
  }
  return values;
}

function namedFunction(sourceFile, name) {
  return sourceFile.statements.filter(statement =>
    ts.isFunctionDeclaration(statement) && statement.name?.text === name);
}

function parameter(functionNode, name) {
  return functionNode.parameters.find(candidate => ts.isIdentifier(candidate.name) && candidate.name.text === name);
}

function typeText(node, sourceFile) {
  return node.type?.getText(sourceFile) ?? '';
}

function descendants(node, predicate) {
  const matches = [];
  function visit(candidate) {
    if (predicate(candidate)) matches.push(candidate);
    ts.forEachChild(candidate, visit);
  }
  visit(node);
  return matches;
}

function callableTypeUsers(sourceFile, typeName) {
  return descendants(sourceFile, node => {
    if (!ts.isParameter(node) || node.type?.getText(sourceFile) !== typeName) return false;
    return ts.isFunctionDeclaration(node.parent)
      || ts.isFunctionExpression(node.parent)
      || ts.isArrowFunction(node.parent)
      || ts.isMethodDeclaration(node.parent);
  });
}

let options;
try {
  options = parseArguments(process.argv.slice(2));
} catch (error) {
  fail(error instanceof Error ? error.message : String(error));
  process.exit();
}

const sourcePath = resolve(options.get('--source'));
let source;
try {
  source = await readFile(sourcePath, 'utf8');
} catch (error) {
  fail(`cannot read source: ${error instanceof Error ? error.message : String(error)}`);
  process.exit();
}

const sourceFile = ts.createSourceFile(sourcePath, source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TS);
if (sourceFile.parseDiagnostics.length > 0) {
  fail(`source contains ${sourceFile.parseDiagnostics.length} TypeScript syntax diagnostic(s)`);
  process.exit();
}
const functionName = options.get('--function');
const functions = namedFunction(sourceFile, functionName);
if (functions.length !== 1) {
  fail(`expected exactly one function declaration named ${functionName}; found ${functions.length}`);
  process.exit();
}

const target = functions[0];
const rootName = options.get('--root');
const rootParameter = parameter(target, rootName);
if (!rootParameter || typeText(rootParameter, sourceFile) !== options.get('--root-type')) {
  fail(`${functionName} must declare ${rootName}: ${options.get('--root-type')}`);
}

const secondaryName = options.get('--parameter');
const secondaryParameter = parameter(target, secondaryName);
if (!secondaryParameter || typeText(secondaryParameter, sourceFile) !== options.get('--parameter-type')) {
  fail(`${functionName} must declare ${secondaryName}: ${options.get('--parameter-type')}`);
}

const mountCalls = descendants(target.body, node =>
  ts.isCallExpression(node)
  && ts.isPropertyAccessExpression(node.expression)
  && node.expression.name.text === 'mount');
if (mountCalls.length !== 1) {
  fail(`${functionName} must contain exactly one .mount() call; found ${mountCalls.length}`);
} else {
  const [mountCall] = mountCalls;
  if (mountCall.arguments.length !== 1
    || !ts.isIdentifier(mountCall.arguments[0])
    || mountCall.arguments[0].text !== rootName) {
    fail(`${functionName} must pass only its ${rootName} parameter binding to .mount()`);
  }
}

const allMountCalls = descendants(sourceFile, node =>
  ts.isCallExpression(node)
  && ts.isPropertyAccessExpression(node.expression)
  && node.expression.name.text === 'mount');
if (allMountCalls.length !== 1 || allMountCalls[0] !== mountCalls[0]) {
  fail('the checked function must own the source file\'s only .mount() call');
}

const implicitDomLookups = descendants(sourceFile, node =>
  ts.isCallExpression(node)
  && ts.isPropertyAccessExpression(node.expression)
  && ts.isIdentifier(node.expression.expression)
  && node.expression.expression.text === 'document'
  && ['querySelector', 'getElementById'].includes(node.expression.name.text));
if (implicitDomLookups.length > 0) {
  fail('implicit document root lookup is prohibited');
}

const unexpectedParameterTypeUses = descendants(sourceFile, node =>
  ts.isTypeReferenceNode(node)
  && node.typeName.getText(sourceFile) === options.get('--parameter-type')
  && node.parent !== secondaryParameter);
if (unexpectedParameterTypeUses.length > 0) {
  fail(`${options.get('--parameter-type')} may be used as a parameter type only by ${functionName}`);
}

const requiredInterface = options.get('--require-interface');
if (requiredInterface) {
  const interfaces = sourceFile.statements.filter(statement =>
    ts.isInterfaceDeclaration(statement) && statement.name.text === requiredInterface);
  if (interfaces.length !== 1) {
    fail(`expected exactly one interface declaration named ${requiredInterface}; found ${interfaces.length}`);
  }
}

const exclusiveFunction = options.get('--exclusive-callable-type-user');
if (exclusiveFunction) {
  const users = callableTypeUsers(sourceFile, options.get('--parameter-type'));
  const owningFunctions = users.map(user =>
    ts.isFunctionDeclaration(user.parent) ? user.parent.name?.text ?? '' : 'non-declaration');
  if (users.length !== 1 || owningFunctions[0] !== exclusiveFunction) {
    fail(`${options.get('--parameter-type')} must have ${exclusiveFunction} as its only callable user`);
  }
}

if (process.exitCode) process.exit();
process.stdout.write(
  `frontend mount contract check: PASS: ${options.get('--kind')} ${functionName} `
  + `binds ${rootName}: ${options.get('--root-type')} and ${secondaryName}: ${options.get('--parameter-type')} `
  + `to .mount(${rootName})\n`,
);
