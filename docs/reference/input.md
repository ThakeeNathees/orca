# input

The `input` block defines an external input parameter for your Orca project.

## Syntax

```orca
input <name> {
  type      = <type>
  desc      = <string>   // optional
  default   = <value>    // optional
  sensitive = <bool>     // optional
}
```

## Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | `schema` | Yes | The type of the input (primitive, schema reference, or inline schema) |
| `desc` | `string \| null` | No | Description of the input |
| `default` | `any \| null` | No | Default value if not provided at runtime |
| `sensitive` | `bool \| null` | No | Whether this input contains sensitive data (e.g., API keys) |

## Examples

### Simple input

```orca
input api_key {
  type      = string
  desc      = "OpenAI API key"
  sensitive = true
}
```

### Input with a schema type

```orca
schema vpc_config {
  region = string
  cidr   = string
}

input network {
  type    = vpc_config
  desc    = "VPC configuration for deployment"
}
```

### Input with inline schema

```orca
input config {
  type = schema {
    region   = string
    replicas = int
    debug    = bool | null
  }
  desc = "Runtime configuration"
}
```
