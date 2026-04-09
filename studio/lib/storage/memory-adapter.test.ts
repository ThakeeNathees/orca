import { runStorageAdapterContract } from "./adapter-contract";
import { MemoryStorageAdapter } from "./memory-adapter";

runStorageAdapterContract("MemoryStorageAdapter", () => new MemoryStorageAdapter());
