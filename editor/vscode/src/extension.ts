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
  };

  return new LanguageClient(
    "orca-lsp",
    "Orca Language Server",
    serverOptions,
    clientOptions
  );
}

export function activate(context: vscode.ExtensionContext) {
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
}

export function deactivate(): Thenable<void> | undefined {
  if (client) {
    return client.stop();
  }
  return undefined;
}
