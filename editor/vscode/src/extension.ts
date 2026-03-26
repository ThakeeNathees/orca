import * as vscode from "vscode";
import {
  LanguageClient,
  LanguageClientOptions,
  ServerOptions,
} from "vscode-languageclient/node";

let client: LanguageClient | undefined;

function createClient(): LanguageClient {
  const config = vscode.workspace.getConfiguration("orca.lsp");
  const orcaPath = config.get<string>("path", "orca");

  const serverOptions: ServerOptions = {
    command: orcaPath,
    args: ["lsp"],
  };

  const clientOptions: LanguageClientOptions = {
    documentSelector: [{ scheme: "file", language: "orca" }],
    outputChannel: outputChannel,
  };

  return new LanguageClient(
    "orca-lsp",
    "Orca Language Server",
    serverOptions,
    clientOptions
  );
}

// Shared output channel so restarts don't create duplicate entries.
let outputChannel: vscode.OutputChannel | undefined;

export function activate(context: vscode.ExtensionContext) {
  outputChannel = vscode.window.createOutputChannel("Orca Language Server");
  context.subscriptions.push(outputChannel);

  client = createClient();
  client.start();

  const restartCmd = vscode.commands.registerCommand(
    "orca.restartLsp",
    async () => {
      if (client) {
        await client.stop();
      }
      client = createClient();
      await client.start();
      vscode.window.showInformationMessage("Orca LSP restarted.");
    }
  );

  context.subscriptions.push(restartCmd);

  // Auto-close triple backticks via completion provider.
  // When the user types ` after ``, offer a completion that expands to
  // the full raw string template with closing ``` and cursor in the middle.
  const rawStringCompleter = vscode.languages.registerCompletionItemProvider(
    "orca",
    {
      provideCompletionItems(document, position) {
        const lineText = document.lineAt(position.line).text;
        const textBefore = lineText.substring(0, position.character);

        // Only trigger when the line ends with ``` (just typed the third backtick).
        if (!textBefore.endsWith("```")) return undefined;

        // Don't trigger on a standalone closing ``` line.
        const beforeBackticks = textBefore.slice(0, -3).trimEnd();
        if (beforeBackticks.length === 0) return undefined;

        const indent = lineText.match(/^(\s*)/)?.[1] ?? "";
        const item = new vscode.CompletionItem(
          "``` raw string",
          vscode.CompletionItemKind.Snippet
        );
        item.detail = "Triple-backtick raw string";
        // Replace the ``` the user already typed, then expand the snippet.
        item.range = new vscode.Range(
          position.translate(0, -3),
          position
        );
        item.insertText = new vscode.SnippetString(
          "```${1:md}\n" + indent + "    $0\n" + indent + "```"
        );
        item.sortText = "!0"; // sort to top
        return [item];
      },
    },
    "`" // trigger on backtick
  );
  context.subscriptions.push(rawStringCompleter);
}

export function deactivate(): Thenable<void> | undefined {
  if (client) {
    return client.stop();
  }
  return undefined;
}
