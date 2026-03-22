# TODO

  (gpt4 → model block) nor member access (gpt4.model_name → string) is implemented.

  To make this work you'd need:

  1. Symbol table — a map of block names to their types/schemas, built by walking all BlockStatements before analyzing assignments
  2. Identifier resolution — ExprType for Identifier looks up the name in the symbol table, returns BlockRef(model) if found
  3. Member access resolution — given the object's resolved type (e.g. BlockRef(model)), look up the field in that block's schema to get the field type
