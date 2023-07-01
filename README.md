# bkl

bkl (short for Baklava, because it has layers) is a templating configuration language without the templates. It's designed to be simple to read and write with obvious behavior.

Write your configuration in your favorite format: [JSON](https://json.org), [YAML](https://yaml.org/), or [TOML](https://toml.io). Layer configurations on top of each other, even in different formats. Use filenames to define the inheritance. Have as many layers as you like. bkl merges your layers together with sane defaults that you can [override](#merge-behavior). Export your results in [any supported format](#output-formats) for human or machine consumption. Use the CLI directly or in scripts, or automate with the library.

No template tags. No schemas. No new formats to learn.

## Example

`service.yaml`
```yaml
name: myService
addr: 127.0.0.1
port: 8080
```

`service.test.toml`
```toml
port = 8081
```

### Run it!
```console
$ bkl service.test.toml
{ "addr": "127.0.0.1", "name": "myService", "port": 8081 }
```

bkl knows that `service.test.toml` inherits from `service.yaml` by the filename pattern, and uses filename extensions to determine formats.

## Output Formats

Output defaults to machine-friendly JSON (you can make that explicit with `-f json`).

### YAML
```console
$ bkl -f yaml service.test.toml
addr: 127.0.0.1
name: myService
port: 8081
```

### TOML
```console
$ bkl -f toml service.test.toml
addr = "127.0.0.1"
name = "myService"
port = 8081
```

### Pretty JSON
```console
$ bkl -f json-pretty service.test.toml
{
  "addr": "127.0.0.1",
  "name": "myService",
  "port": 8081
}
```

## Output Locations

Output goes to stdout by default. Errors always go to stderr.

### File Output
```console
$ bkl -o out.yaml service.test.toml
```

Output format is autodetected from output filename.

## Advanced Inputs

### Multiple Files

Specifying multiple input files evaluates them as normal, then merges them onto each other in order.

```console
$ bkl a.b.yaml c.d.yaml   # (a.yaml + a.b.yaml) + (c.yaml + c.d.yaml)
```

### Symlinks

bkl follows symbolic links and evaluates the inherited layers on the other side of the symlink.

```console
$ ln -s a.b.yaml c.yaml
$ bkl c.d.yaml   # a.yaml + a.b.yaml (c.yaml) + c.d.yaml

```

### Streams

bkl understands input streams (multi-document YAML files delimited with `---`). To layer them, it has to match up sections between files. It tries the following strategies, in order:
* `$match`: specify match fields in the document:
```yaml
$match:
  kind: Service
  metadata:
    name: myService
```
* K8s paths: If `kind` and `metadata.name` are present, they are used as default match keys.
* Ordering: Stream position is used to match documents.

## Merge Behavior

By default, lists and maps are merged. To change that, use [$patch](https://github.com/edgarsandi/Kubernetes/blob/master/docs/devel/api-conventions.md#strategic-merge-patch) syntax.

### Maps

<table>
  
<tr>

<td>

```yaml
myMap:
  a: 1
```
</td>

<td>

**+**
</td>

<td>

```yaml
myMap:
  b: 2
```
</td>

<td>

**=**
</td>

<td>

```yaml
myMap:
  a: 1
  b: 2
```
</td>

</tr>

<tr></tr>

<tr>

<td>

```yaml
myMap:
  a: 1
```
</td>

<td>

**+**
</td>

<td>

```yaml
myMap:
  b: 2
  $patch: replace
```
</td>

<td>

**=**
</td>

<td>

```yaml
myMap:
  b: 2
```
</td>

</tr>

<tr></tr>

<tr>

<td>

```yaml
myMap:
  a: 1
  b: 2
```
</td>

<td>

**+**
</td>

<td>

```yaml
myMap:
  c: 3
  b: null
```
</td>

<td>

**=**
</td>

<td>

```yaml
myMap:
  a: 1
  c: 3
```
</td>

</tr>

</table>

### Lists

<table>
  
<tr>

<td>

```yaml
myList:
  - 1
```
</td>

<td>

**+**
</td>

<td>

```yaml
myList:
  - 2
```
</td>

<td>

**=**
</td>

<td>

```yaml
myList:
  - 1
  - 2
```
</td>

</tr>

<tr></tr>

<tr>

<td>

```yaml
myList:
  - 1
```
</td>

<td>

**+**
</td>

<td>

```yaml
myList:
  - 2
  - $patch: replace
```
</td>

<td>

**=**
</td>

<td>

```yaml
myList:
  - 2
```
</td>

</tr>

<tr></tr>

<tr>

<td>

```yaml
myList:
  - x: 1
  - x: 2
```
</td>

<td>

**+**
</td>

<td>

```yaml
myList:
  - x: 3
  - x: 2
    $patch: delete
```
</td>

<td>

**=**
</td>

<td>

```yaml
myList:
  - x: 1
  - x: 3
```
</td>

</tr>

</table>

## Advanced Values

### $required

Use `$required` in lower layers to force upper layers to replace the value.

```yaml
myMap:
  a: 1
  b: $required
```

This will cause an error unless a layer on top sets `myMap.b`.

### $merge

Merges in another subtree.

```yaml
foo:
  bar:
    a: 1
zig:
  b: 2
  $merge: foo.bar
```

evaluates to:

```yaml
foo:
  bar:
    a: 1
zig:
  a: 1
  b: 2
```

### $replace

Replaces a subtree with one from another location.

```yaml
foo:
  bar:
    a: 1
zig:
  b: 2
  $replace: foo.bar
```

evaluates to:

```yaml
foo:
  bar:
    a: 1
zig:
  a: 1
```

### $output

When used at the top level, selects a subtree for output:

```yaml
$output: foo.bar
foo:
  bar:
    a: 1
```

evaluates to:

```yaml
a: 1
```

Combine `$copy` and `$output` to have hidden "template" subtrees that don't appear in the output but can be copied in as needed. 
