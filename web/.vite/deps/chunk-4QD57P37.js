import {
  DescriptorProtoSchema,
  Edition,
  EnumDescriptorProtoSchema,
  FieldDescriptorProtoSchema,
  FieldOptionsSchema,
  FileDescriptorProtoSchema,
  ScalarType,
  base64Encode,
  clearField,
  clone,
  isFieldSet,
  protoCamelCase,
  toBinary
} from "./chunk-5J2BFMBY.js";

// node_modules/@bufbuild/protobuf/dist/esm/codegenv2/embed.js
function embedFileDesc(file) {
  const embed = {
    bootable: false,
    proto() {
      const stripped = clone(FileDescriptorProtoSchema, file);
      clearField(stripped, FileDescriptorProtoSchema.field.dependency);
      clearField(stripped, FileDescriptorProtoSchema.field.sourceCodeInfo);
      stripped.messageType.map(stripJsonNames);
      return stripped;
    },
    base64() {
      const bytes = toBinary(FileDescriptorProtoSchema, this.proto());
      return base64Encode(bytes, "std_raw");
    }
  };
  return file.name == "google/protobuf/descriptor.proto" ? Object.assign(Object.assign({}, embed), { bootable: true, boot() {
    return createFileDescriptorProtoBoot(this.proto());
  } }) : embed;
}
function stripJsonNames(d) {
  for (const f of d.field) {
    if (f.jsonName === protoCamelCase(f.name)) {
      clearField(f, FieldDescriptorProtoSchema.field.jsonName);
    }
  }
  for (const n of d.nestedType) {
    stripJsonNames(n);
  }
}
function pathInFileDesc(desc) {
  if (desc.kind == "service") {
    return [desc.file.services.indexOf(desc)];
  }
  const parent = desc.parent;
  if (parent == void 0) {
    switch (desc.kind) {
      case "enum":
        return [desc.file.enums.indexOf(desc)];
      case "message":
        return [desc.file.messages.indexOf(desc)];
      case "extension":
        return [desc.file.extensions.indexOf(desc)];
    }
  }
  function findPath(cur) {
    const nested = [];
    for (let parent2 = cur.parent; parent2; ) {
      const idx = parent2.nestedMessages.indexOf(cur);
      nested.unshift(idx);
      cur = parent2;
      parent2 = cur.parent;
    }
    nested.unshift(cur.file.messages.indexOf(cur));
    return nested;
  }
  const path = findPath(parent);
  switch (desc.kind) {
    case "extension":
      return [...path, parent.nestedExtensions.indexOf(desc)];
    case "message":
      return [...path, parent.nestedMessages.indexOf(desc)];
    case "enum":
      return [...path, parent.nestedEnums.indexOf(desc)];
  }
}
function createFileDescriptorProtoBoot(proto) {
  var _a;
  assert(proto.name == "google/protobuf/descriptor.proto");
  assert(proto.package == "google.protobuf");
  assert(!proto.dependency.length);
  assert(!proto.publicDependency.length);
  assert(!proto.weakDependency.length);
  assert(!proto.optionDependency.length);
  assert(!proto.service.length);
  assert(!proto.extension.length);
  assert(proto.sourceCodeInfo === void 0);
  assert(proto.syntax == "" || proto.syntax == "proto2");
  assert(!((_a = proto.options) === null || _a === void 0 ? void 0 : _a.features));
  assert(proto.edition === Edition.EDITION_UNKNOWN);
  return {
    name: proto.name,
    package: proto.package,
    messageType: proto.messageType.map(createDescriptorBoot),
    enumType: proto.enumType.map(createEnumDescriptorBoot)
  };
}
function createDescriptorBoot(proto) {
  assert(proto.extension.length == 0);
  assert(!proto.oneofDecl.length);
  assert(!proto.options);
  assert(!isFieldSet(proto, DescriptorProtoSchema.field.visibility));
  const b = {
    name: proto.name
  };
  if (proto.field.length) {
    b.field = proto.field.map(createFieldDescriptorBoot);
  }
  if (proto.nestedType.length) {
    b.nestedType = proto.nestedType.map(createDescriptorBoot);
  }
  if (proto.enumType.length) {
    b.enumType = proto.enumType.map(createEnumDescriptorBoot);
  }
  if (proto.extensionRange.length) {
    b.extensionRange = proto.extensionRange.map((r) => {
      assert(!r.options);
      return { start: r.start, end: r.end };
    });
  }
  return b;
}
function createFieldDescriptorBoot(proto) {
  assert(isFieldSet(proto, FieldDescriptorProtoSchema.field.name));
  assert(isFieldSet(proto, FieldDescriptorProtoSchema.field.number));
  assert(isFieldSet(proto, FieldDescriptorProtoSchema.field.type));
  assert(!isFieldSet(proto, FieldDescriptorProtoSchema.field.oneofIndex));
  assert(!isFieldSet(proto, FieldDescriptorProtoSchema.field.jsonName) || proto.jsonName === protoCamelCase(proto.name));
  const b = {
    name: proto.name,
    number: proto.number,
    type: proto.type
  };
  if (isFieldSet(proto, FieldDescriptorProtoSchema.field.label)) {
    b.label = proto.label;
  }
  if (isFieldSet(proto, FieldDescriptorProtoSchema.field.typeName)) {
    b.typeName = proto.typeName;
  }
  if (isFieldSet(proto, FieldDescriptorProtoSchema.field.extendee)) {
    b.extendee = proto.extendee;
  }
  if (isFieldSet(proto, FieldDescriptorProtoSchema.field.defaultValue)) {
    b.defaultValue = proto.defaultValue;
  }
  if (proto.options) {
    b.options = createFieldOptionsBoot(proto.options);
  }
  return b;
}
function createFieldOptionsBoot(proto) {
  const b = {};
  assert(!isFieldSet(proto, FieldOptionsSchema.field.ctype));
  if (isFieldSet(proto, FieldOptionsSchema.field.packed)) {
    b.packed = proto.packed;
  }
  assert(!isFieldSet(proto, FieldOptionsSchema.field.jstype));
  assert(!isFieldSet(proto, FieldOptionsSchema.field.lazy));
  assert(!isFieldSet(proto, FieldOptionsSchema.field.unverifiedLazy));
  if (isFieldSet(proto, FieldOptionsSchema.field.deprecated)) {
    b.deprecated = proto.deprecated;
  }
  assert(!isFieldSet(proto, FieldOptionsSchema.field.weak));
  assert(!isFieldSet(proto, FieldOptionsSchema.field.debugRedact));
  if (isFieldSet(proto, FieldOptionsSchema.field.retention)) {
    b.retention = proto.retention;
  }
  if (proto.targets.length) {
    b.targets = proto.targets;
  }
  if (proto.editionDefaults.length) {
    b.editionDefaults = proto.editionDefaults.map((d) => ({
      value: d.value,
      edition: d.edition
    }));
  }
  assert(!isFieldSet(proto, FieldOptionsSchema.field.features));
  assert(!isFieldSet(proto, FieldOptionsSchema.field.uninterpretedOption));
  return b;
}
function createEnumDescriptorBoot(proto) {
  assert(!proto.options);
  assert(!isFieldSet(proto, EnumDescriptorProtoSchema.field.visibility));
  return {
    name: proto.name,
    value: proto.value.map((v) => {
      assert(!v.options);
      return {
        name: v.name,
        number: v.number
      };
    })
  };
}
function assert(condition) {
  if (!condition) {
    throw new Error();
  }
}

// node_modules/@bufbuild/protobuf/dist/esm/codegenv2/service.js
function serviceDesc(file, path, ...paths) {
  if (paths.length > 0) {
    throw new Error();
  }
  return file.services[path];
}

// node_modules/@bufbuild/protobuf/dist/esm/codegenv2/symbols.js
var packageName = "@bufbuild/protobuf";
var wktPublicImportPaths = {
  "google/protobuf/compiler/plugin.proto": packageName + "/wkt",
  "google/protobuf/any.proto": packageName + "/wkt",
  "google/protobuf/api.proto": packageName + "/wkt",
  "google/protobuf/cpp_features.proto": packageName + "/wkt",
  "google/protobuf/descriptor.proto": packageName + "/wkt",
  "google/protobuf/duration.proto": packageName + "/wkt",
  "google/protobuf/empty.proto": packageName + "/wkt",
  "google/protobuf/field_mask.proto": packageName + "/wkt",
  "google/protobuf/go_features.proto": packageName + "/wkt",
  "google/protobuf/java_features.proto": packageName + "/wkt",
  "google/protobuf/source_context.proto": packageName + "/wkt",
  "google/protobuf/struct.proto": packageName + "/wkt",
  "google/protobuf/timestamp.proto": packageName + "/wkt",
  "google/protobuf/type.proto": packageName + "/wkt",
  "google/protobuf/wrappers.proto": packageName + "/wkt"
};
var symbols = {
  isMessage: { typeOnly: false, bootstrapWktFrom: "../../is-message.js", from: packageName },
  Message: { typeOnly: true, bootstrapWktFrom: "../../types.js", from: packageName },
  create: { typeOnly: false, bootstrapWktFrom: "../../create.js", from: packageName },
  fromJson: { typeOnly: false, bootstrapWktFrom: "../../from-json.js", from: packageName },
  fromJsonString: { typeOnly: false, bootstrapWktFrom: "../../from-json.js", from: packageName },
  fromBinary: { typeOnly: false, bootstrapWktFrom: "../../from-binary.js", from: packageName },
  toBinary: { typeOnly: false, bootstrapWktFrom: "../../to-binary.js", from: packageName },
  toJson: { typeOnly: false, bootstrapWktFrom: "../../to-json.js", from: packageName },
  toJsonString: { typeOnly: false, bootstrapWktFrom: "../../to-json.js", from: packageName },
  protoInt64: { typeOnly: false, bootstrapWktFrom: "../../proto-int64.js", from: packageName },
  JsonValue: { typeOnly: true, bootstrapWktFrom: "../../json-value.js", from: packageName },
  JsonObject: { typeOnly: true, bootstrapWktFrom: "../../json-value.js", from: packageName },
  UnknownEnum: { typeOnly: true, bootstrapWktFrom: "../../types.js", from: packageName },
  codegen: {
    boot: { typeOnly: false, bootstrapWktFrom: "../../codegenv2/boot.js", from: packageName + "/codegenv2" },
    fileDesc: { typeOnly: false, bootstrapWktFrom: "../../codegenv2/file.js", from: packageName + "/codegenv2" },
    enumDesc: { typeOnly: false, bootstrapWktFrom: "../../codegenv2/enum.js", from: packageName + "/codegenv2" },
    extDesc: { typeOnly: false, bootstrapWktFrom: "../../codegenv2/extension.js", from: packageName + "/codegenv2" },
    messageDesc: { typeOnly: false, bootstrapWktFrom: "../../codegenv2/message.js", from: packageName + "/codegenv2" },
    serviceDesc: { typeOnly: false, bootstrapWktFrom: "../../codegenv2/service.js", from: packageName + "/codegenv2" },
    tsEnum: { typeOnly: false, bootstrapWktFrom: "../../codegenv2/enum.js", from: packageName + "/codegenv2" },
    objEnum: { typeOnly: false, bootstrapWktFrom: "../../codegenv2/enum.js", from: packageName + "/codegenv2" },
    GenFile: { typeOnly: true, bootstrapWktFrom: "../../codegenv2/types.js", from: packageName + "/codegenv2" },
    GenEnum: { typeOnly: true, bootstrapWktFrom: "../../codegenv2/types.js", from: packageName + "/codegenv2" },
    GenExtension: { typeOnly: true, bootstrapWktFrom: "../../codegenv2/types.js", from: packageName + "/codegenv2" },
    GenMessage: { typeOnly: true, bootstrapWktFrom: "../../codegenv2/types.js", from: packageName + "/codegenv2" },
    GenService: { typeOnly: true, bootstrapWktFrom: "../../codegenv2/types.js", from: packageName + "/codegenv2" }
  }
};

// node_modules/@bufbuild/protobuf/dist/esm/codegenv2/scalar.js
function scalarTypeScriptType(scalar, longAsString) {
  switch (scalar) {
    case ScalarType.STRING:
      return "string";
    case ScalarType.BOOL:
      return "boolean";
    case ScalarType.UINT64:
    case ScalarType.SFIXED64:
    case ScalarType.FIXED64:
    case ScalarType.SINT64:
    case ScalarType.INT64:
      return longAsString ? "string" : "bigint";
    case ScalarType.BYTES:
      return "Uint8Array";
    default:
      return "number";
  }
}
function scalarJsonType(scalar) {
  switch (scalar) {
    case ScalarType.DOUBLE:
    case ScalarType.FLOAT:
      return `number | "NaN" | "Infinity" | "-Infinity"`;
    case ScalarType.UINT64:
    case ScalarType.SFIXED64:
    case ScalarType.FIXED64:
    case ScalarType.SINT64:
    case ScalarType.INT64:
      return "string";
    case ScalarType.INT32:
    case ScalarType.FIXED32:
    case ScalarType.UINT32:
    case ScalarType.SFIXED32:
    case ScalarType.SINT32:
      return "number";
    case ScalarType.STRING:
      return "string";
    case ScalarType.BOOL:
      return "boolean";
    case ScalarType.BYTES:
      return "string";
  }
}

export {
  embedFileDesc,
  pathInFileDesc,
  createFileDescriptorProtoBoot,
  serviceDesc,
  packageName,
  wktPublicImportPaths,
  symbols,
  scalarTypeScriptType,
  scalarJsonType
};
//# sourceMappingURL=chunk-4QD57P37.js.map
