import {
  BinaryReader,
  BinaryWriter,
  FieldError,
  ScalarType,
  base64Decode,
  base64Encode,
  checkField,
  create,
  enumDesc,
  extDesc,
  fileDesc,
  file_google_protobuf_descriptor,
  formatVal,
  fromBinary,
  hasCustomJsonRepresentation,
  isFieldError,
  isWrapperDesc,
  makeReadContext,
  messageDesc,
  protoCamelCase,
  protoInt64,
  protoSnakeCase,
  readField,
  reflect,
  scalarEquals,
  scalarZeroValue,
  toBinary,
  writeField
} from "./chunk-5J2BFMBY.js";

// node_modules/@bufbuild/protobuf/dist/esm/wkt/gen/google/protobuf/timestamp_pb.js
var file_google_protobuf_timestamp = fileDesc("Ch9nb29nbGUvcHJvdG9idWYvdGltZXN0YW1wLnByb3RvEg9nb29nbGUucHJvdG9idWYiKwoJVGltZXN0YW1wEg8KB3NlY29uZHMYASABKAMSDQoFbmFub3MYAiABKAVChQEKE2NvbS5nb29nbGUucHJvdG9idWZCDlRpbWVzdGFtcFByb3RvUAFaMmdvb2dsZS5nb2xhbmcub3JnL3Byb3RvYnVmL3R5cGVzL2tub3duL3RpbWVzdGFtcHBi+AEBogIDR1BCqgIeR29vZ2xlLlByb3RvYnVmLldlbGxLbm93blR5cGVzYgZwcm90bzM");
var TimestampSchema = messageDesc(file_google_protobuf_timestamp, 0);

// node_modules/@bufbuild/protobuf/dist/esm/wkt/gen/google/protobuf/duration_pb.js
var file_google_protobuf_duration = fileDesc("Ch5nb29nbGUvcHJvdG9idWYvZHVyYXRpb24ucHJvdG8SD2dvb2dsZS5wcm90b2J1ZiIqCghEdXJhdGlvbhIPCgdzZWNvbmRzGAEgASgDEg0KBW5hbm9zGAIgASgFQoMBChNjb20uZ29vZ2xlLnByb3RvYnVmQg1EdXJhdGlvblByb3RvUAFaMWdvb2dsZS5nb2xhbmcub3JnL3Byb3RvYnVmL3R5cGVzL2tub3duL2R1cmF0aW9ucGL4AQGiAgNHUEKqAh5Hb29nbGUuUHJvdG9idWYuV2VsbEtub3duVHlwZXNiBnByb3RvMw");
var DurationSchema = messageDesc(file_google_protobuf_duration, 0);

// node_modules/@bufbuild/protobuf/dist/esm/wkt/gen/google/protobuf/any_pb.js
var file_google_protobuf_any = fileDesc("Chlnb29nbGUvcHJvdG9idWYvYW55LnByb3RvEg9nb29nbGUucHJvdG9idWYiJgoDQW55EhAKCHR5cGVfdXJsGAEgASgJEg0KBXZhbHVlGAIgASgMQnYKE2NvbS5nb29nbGUucHJvdG9idWZCCEFueVByb3RvUAFaLGdvb2dsZS5nb2xhbmcub3JnL3Byb3RvYnVmL3R5cGVzL2tub3duL2FueXBiogIDR1BCqgIeR29vZ2xlLlByb3RvYnVmLldlbGxLbm93blR5cGVzYgZwcm90bzM");
var AnySchema = messageDesc(file_google_protobuf_any, 0);

// node_modules/@bufbuild/protobuf/dist/esm/wkt/any.js
function anyPack(schema, message, into) {
  let ret = false;
  if (!into) {
    into = create(AnySchema);
    ret = true;
  }
  into.value = toBinary(schema, message);
  into.typeUrl = typeNameToUrl(message.$typeName);
  return ret ? into : void 0;
}
function anyIs(any, descOrTypeName) {
  if (any.typeUrl === "") {
    return false;
  }
  const want = typeof descOrTypeName == "string" ? descOrTypeName : descOrTypeName.typeName;
  const got = typeUrlToName(any.typeUrl);
  return want === got;
}
function anyUnpack(any, registryOrMessageDesc) {
  if (any.typeUrl === "") {
    return void 0;
  }
  const desc = registryOrMessageDesc.kind == "message" ? registryOrMessageDesc : registryOrMessageDesc.getMessage(typeUrlToName(any.typeUrl));
  if (!desc || !anyIs(any, desc)) {
    return void 0;
  }
  return fromBinary(desc, any.value);
}
function typeNameToUrl(name) {
  return `type.googleapis.com/${name}`;
}
function typeUrlToName(url) {
  const slash = url.lastIndexOf("/");
  const name = slash >= 0 ? url.substring(slash + 1) : url;
  if (!name.length) {
    throw new Error(`invalid type url: ${url}`);
  }
  return name;
}

// node_modules/@bufbuild/protobuf/dist/esm/wkt/gen/google/protobuf/source_context_pb.js
var file_google_protobuf_source_context = fileDesc("CiRnb29nbGUvcHJvdG9idWYvc291cmNlX2NvbnRleHQucHJvdG8SD2dvb2dsZS5wcm90b2J1ZiIiCg1Tb3VyY2VDb250ZXh0EhEKCWZpbGVfbmFtZRgBIAEoCUKKAQoTY29tLmdvb2dsZS5wcm90b2J1ZkISU291cmNlQ29udGV4dFByb3RvUAFaNmdvb2dsZS5nb2xhbmcub3JnL3Byb3RvYnVmL3R5cGVzL2tub3duL3NvdXJjZWNvbnRleHRwYqICA0dQQqoCHkdvb2dsZS5Qcm90b2J1Zi5XZWxsS25vd25UeXBlc2IGcHJvdG8z");
var SourceContextSchema = messageDesc(file_google_protobuf_source_context, 0);

// node_modules/@bufbuild/protobuf/dist/esm/wkt/gen/google/protobuf/type_pb.js
var file_google_protobuf_type = fileDesc("Chpnb29nbGUvcHJvdG9idWYvdHlwZS5wcm90bxIPZ29vZ2xlLnByb3RvYnVmIugBCgRUeXBlEgwKBG5hbWUYASABKAkSJgoGZmllbGRzGAIgAygLMhYuZ29vZ2xlLnByb3RvYnVmLkZpZWxkEg4KBm9uZW9mcxgDIAMoCRIoCgdvcHRpb25zGAQgAygLMhcuZ29vZ2xlLnByb3RvYnVmLk9wdGlvbhI2Cg5zb3VyY2VfY29udGV4dBgFIAEoCzIeLmdvb2dsZS5wcm90b2J1Zi5Tb3VyY2VDb250ZXh0EicKBnN5bnRheBgGIAEoDjIXLmdvb2dsZS5wcm90b2J1Zi5TeW50YXgSDwoHZWRpdGlvbhgHIAEoCSLVBQoFRmllbGQSKQoEa2luZBgBIAEoDjIbLmdvb2dsZS5wcm90b2J1Zi5GaWVsZC5LaW5kEjcKC2NhcmRpbmFsaXR5GAIgASgOMiIuZ29vZ2xlLnByb3RvYnVmLkZpZWxkLkNhcmRpbmFsaXR5Eg4KBm51bWJlchgDIAEoBRIMCgRuYW1lGAQgASgJEhAKCHR5cGVfdXJsGAYgASgJEhMKC29uZW9mX2luZGV4GAcgASgFEg4KBnBhY2tlZBgIIAEoCBIoCgdvcHRpb25zGAkgAygLMhcuZ29vZ2xlLnByb3RvYnVmLk9wdGlvbhIRCglqc29uX25hbWUYCiABKAkSFQoNZGVmYXVsdF92YWx1ZRgLIAEoCSLIAgoES2luZBIQCgxUWVBFX1VOS05PV04QABIPCgtUWVBFX0RPVUJMRRABEg4KClRZUEVfRkxPQVQQAhIOCgpUWVBFX0lOVDY0EAMSDwoLVFlQRV9VSU5UNjQQBBIOCgpUWVBFX0lOVDMyEAUSEAoMVFlQRV9GSVhFRDY0EAYSEAoMVFlQRV9GSVhFRDMyEAcSDQoJVFlQRV9CT09MEAgSDwoLVFlQRV9TVFJJTkcQCRIOCgpUWVBFX0dST1VQEAoSEAoMVFlQRV9NRVNTQUdFEAsSDgoKVFlQRV9CWVRFUxAMEg8KC1RZUEVfVUlOVDMyEA0SDQoJVFlQRV9FTlVNEA4SEQoNVFlQRV9TRklYRUQzMhAPEhEKDVRZUEVfU0ZJWEVENjQQEBIPCgtUWVBFX1NJTlQzMhAREg8KC1RZUEVfU0lOVDY0EBIidAoLQ2FyZGluYWxpdHkSFwoTQ0FSRElOQUxJVFlfVU5LTk9XThAAEhgKFENBUkRJTkFMSVRZX09QVElPTkFMEAESGAoUQ0FSRElOQUxJVFlfUkVRVUlSRUQQAhIYChRDQVJESU5BTElUWV9SRVBFQVRFRBADIt8BCgRFbnVtEgwKBG5hbWUYASABKAkSLQoJZW51bXZhbHVlGAIgAygLMhouZ29vZ2xlLnByb3RvYnVmLkVudW1WYWx1ZRIoCgdvcHRpb25zGAMgAygLMhcuZ29vZ2xlLnByb3RvYnVmLk9wdGlvbhI2Cg5zb3VyY2VfY29udGV4dBgEIAEoCzIeLmdvb2dsZS5wcm90b2J1Zi5Tb3VyY2VDb250ZXh0EicKBnN5bnRheBgFIAEoDjIXLmdvb2dsZS5wcm90b2J1Zi5TeW50YXgSDwoHZWRpdGlvbhgGIAEoCSJTCglFbnVtVmFsdWUSDAoEbmFtZRgBIAEoCRIOCgZudW1iZXIYAiABKAUSKAoHb3B0aW9ucxgDIAMoCzIXLmdvb2dsZS5wcm90b2J1Zi5PcHRpb24iOwoGT3B0aW9uEgwKBG5hbWUYASABKAkSIwoFdmFsdWUYAiABKAsyFC5nb29nbGUucHJvdG9idWYuQW55KkMKBlN5bnRheBIRCg1TWU5UQVhfUFJPVE8yEAASEQoNU1lOVEFYX1BST1RPMxABEhMKD1NZTlRBWF9FRElUSU9OUxACQnsKE2NvbS5nb29nbGUucHJvdG9idWZCCVR5cGVQcm90b1ABWi1nb29nbGUuZ29sYW5nLm9yZy9wcm90b2J1Zi90eXBlcy9rbm93bi90eXBlcGL4AQGiAgNHUEKqAh5Hb29nbGUuUHJvdG9idWYuV2VsbEtub3duVHlwZXNiBnByb3RvMw", [file_google_protobuf_any, file_google_protobuf_source_context]);
var TypeSchema = messageDesc(file_google_protobuf_type, 0);
var FieldSchema = messageDesc(file_google_protobuf_type, 1);
var Field_Kind;
(function(Field_Kind2) {
  Field_Kind2[Field_Kind2["TYPE_UNKNOWN"] = 0] = "TYPE_UNKNOWN";
  Field_Kind2[Field_Kind2["TYPE_DOUBLE"] = 1] = "TYPE_DOUBLE";
  Field_Kind2[Field_Kind2["TYPE_FLOAT"] = 2] = "TYPE_FLOAT";
  Field_Kind2[Field_Kind2["TYPE_INT64"] = 3] = "TYPE_INT64";
  Field_Kind2[Field_Kind2["TYPE_UINT64"] = 4] = "TYPE_UINT64";
  Field_Kind2[Field_Kind2["TYPE_INT32"] = 5] = "TYPE_INT32";
  Field_Kind2[Field_Kind2["TYPE_FIXED64"] = 6] = "TYPE_FIXED64";
  Field_Kind2[Field_Kind2["TYPE_FIXED32"] = 7] = "TYPE_FIXED32";
  Field_Kind2[Field_Kind2["TYPE_BOOL"] = 8] = "TYPE_BOOL";
  Field_Kind2[Field_Kind2["TYPE_STRING"] = 9] = "TYPE_STRING";
  Field_Kind2[Field_Kind2["TYPE_GROUP"] = 10] = "TYPE_GROUP";
  Field_Kind2[Field_Kind2["TYPE_MESSAGE"] = 11] = "TYPE_MESSAGE";
  Field_Kind2[Field_Kind2["TYPE_BYTES"] = 12] = "TYPE_BYTES";
  Field_Kind2[Field_Kind2["TYPE_UINT32"] = 13] = "TYPE_UINT32";
  Field_Kind2[Field_Kind2["TYPE_ENUM"] = 14] = "TYPE_ENUM";
  Field_Kind2[Field_Kind2["TYPE_SFIXED32"] = 15] = "TYPE_SFIXED32";
  Field_Kind2[Field_Kind2["TYPE_SFIXED64"] = 16] = "TYPE_SFIXED64";
  Field_Kind2[Field_Kind2["TYPE_SINT32"] = 17] = "TYPE_SINT32";
  Field_Kind2[Field_Kind2["TYPE_SINT64"] = 18] = "TYPE_SINT64";
})(Field_Kind || (Field_Kind = {}));
var Field_KindSchema = enumDesc(file_google_protobuf_type, 1, 0);
var Field_Cardinality;
(function(Field_Cardinality2) {
  Field_Cardinality2[Field_Cardinality2["UNKNOWN"] = 0] = "UNKNOWN";
  Field_Cardinality2[Field_Cardinality2["OPTIONAL"] = 1] = "OPTIONAL";
  Field_Cardinality2[Field_Cardinality2["REQUIRED"] = 2] = "REQUIRED";
  Field_Cardinality2[Field_Cardinality2["REPEATED"] = 3] = "REPEATED";
})(Field_Cardinality || (Field_Cardinality = {}));
var Field_CardinalitySchema = enumDesc(file_google_protobuf_type, 1, 1);
var EnumSchema = messageDesc(file_google_protobuf_type, 2);
var EnumValueSchema = messageDesc(file_google_protobuf_type, 3);
var OptionSchema = messageDesc(file_google_protobuf_type, 4);
var Syntax;
(function(Syntax2) {
  Syntax2[Syntax2["PROTO2"] = 0] = "PROTO2";
  Syntax2[Syntax2["PROTO3"] = 1] = "PROTO3";
  Syntax2[Syntax2["EDITIONS"] = 2] = "EDITIONS";
})(Syntax || (Syntax = {}));
var SyntaxSchema = enumDesc(file_google_protobuf_type, 0);

// node_modules/@bufbuild/protobuf/dist/esm/wkt/gen/google/protobuf/api_pb.js
var file_google_protobuf_api = fileDesc("Chlnb29nbGUvcHJvdG9idWYvYXBpLnByb3RvEg9nb29nbGUucHJvdG9idWYikgIKA0FwaRIMCgRuYW1lGAEgASgJEigKB21ldGhvZHMYAiADKAsyFy5nb29nbGUucHJvdG9idWYuTWV0aG9kEigKB29wdGlvbnMYAyADKAsyFy5nb29nbGUucHJvdG9idWYuT3B0aW9uEg8KB3ZlcnNpb24YBCABKAkSNgoOc291cmNlX2NvbnRleHQYBSABKAsyHi5nb29nbGUucHJvdG9idWYuU291cmNlQ29udGV4dBImCgZtaXhpbnMYBiADKAsyFi5nb29nbGUucHJvdG9idWYuTWl4aW4SJwoGc3ludGF4GAcgASgOMhcuZ29vZ2xlLnByb3RvYnVmLlN5bnRheBIPCgdlZGl0aW9uGAggASgJIu4BCgZNZXRob2QSDAoEbmFtZRgBIAEoCRIYChByZXF1ZXN0X3R5cGVfdXJsGAIgASgJEhkKEXJlcXVlc3Rfc3RyZWFtaW5nGAMgASgIEhkKEXJlc3BvbnNlX3R5cGVfdXJsGAQgASgJEhoKEnJlc3BvbnNlX3N0cmVhbWluZxgFIAEoCBIoCgdvcHRpb25zGAYgAygLMhcuZ29vZ2xlLnByb3RvYnVmLk9wdGlvbhIrCgZzeW50YXgYByABKA4yFy5nb29nbGUucHJvdG9idWYuU3ludGF4QgIYARITCgdlZGl0aW9uGAggASgJQgIYASIjCgVNaXhpbhIMCgRuYW1lGAEgASgJEgwKBHJvb3QYAiABKAlCdgoTY29tLmdvb2dsZS5wcm90b2J1ZkIIQXBpUHJvdG9QAVosZ29vZ2xlLmdvbGFuZy5vcmcvcHJvdG9idWYvdHlwZXMva25vd24vYXBpcGKiAgNHUEKqAh5Hb29nbGUuUHJvdG9idWYuV2VsbEtub3duVHlwZXNiBnByb3RvMw", [file_google_protobuf_source_context, file_google_protobuf_type]);
var ApiSchema = messageDesc(file_google_protobuf_api, 0);
var MethodSchema = messageDesc(file_google_protobuf_api, 1);
var MixinSchema = messageDesc(file_google_protobuf_api, 2);

// node_modules/@bufbuild/protobuf/dist/esm/wkt/gen/google/protobuf/cpp_features_pb.js
var file_google_protobuf_cpp_features = fileDesc("CiJnb29nbGUvcHJvdG9idWYvY3BwX2ZlYXR1cmVzLnByb3RvEgJwYiL8AwoLQ3BwRmVhdHVyZXMS+wEKEmxlZ2FjeV9jbG9zZWRfZW51bRgBIAEoCELeAYgBAZgBBJgBAaIBCRIEdHJ1ZRiEB6IBChIFZmFsc2UY5weyAbgBCOgHEOgHGq8BVGhlIGxlZ2FjeSBjbG9zZWQgZW51bSBiZWhhdmlvciBpbiBDKysgaXMgZGVwcmVjYXRlZCBhbmQgaXMgc2NoZWR1bGVkIHRvIGJlIHJlbW92ZWQgaW4gZWRpdGlvbiAyMDI1LiAgU2VlIGh0dHA6Ly9wcm90b2J1Zi5kZXYvcHJvZ3JhbW1pbmctZ3VpZGVzL2VudW0vI2NwcCBmb3IgbW9yZSBpbmZvcm1hdGlvbhJaCgtzdHJpbmdfdHlwZRgCIAEoDjIaLnBiLkNwcEZlYXR1cmVzLlN0cmluZ1R5cGVCKYgBAZgBBJgBAaIBCxIGU1RSSU5HGIQHogEJEgRWSUVXGOkHsgEDCOgHEkwKGmVudW1fbmFtZV91c2VzX3N0cmluZ192aWV3GAMgASgIQiiIAQGYAQaYAQGiAQoSBWZhbHNlGIQHogEJEgR0cnVlGOkHsgEDCOkHIkUKClN0cmluZ1R5cGUSFwoTU1RSSU5HX1RZUEVfVU5LTk9XThAAEggKBFZJRVcQARIICgRDT1JEEAISCgoGU1RSSU5HEAM6PwoDY3BwEhsuZ29vZ2xlLnByb3RvYnVmLkZlYXR1cmVTZXQY6AcgASgLMg8ucGIuQ3BwRmVhdHVyZXNSA2NwcA", [file_google_protobuf_descriptor]);
var CppFeaturesSchema = messageDesc(file_google_protobuf_cpp_features, 0);
var CppFeatures_StringType;
(function(CppFeatures_StringType2) {
  CppFeatures_StringType2[CppFeatures_StringType2["STRING_TYPE_UNKNOWN"] = 0] = "STRING_TYPE_UNKNOWN";
  CppFeatures_StringType2[CppFeatures_StringType2["VIEW"] = 1] = "VIEW";
  CppFeatures_StringType2[CppFeatures_StringType2["CORD"] = 2] = "CORD";
  CppFeatures_StringType2[CppFeatures_StringType2["STRING"] = 3] = "STRING";
})(CppFeatures_StringType || (CppFeatures_StringType = {}));
var CppFeatures_StringTypeSchema = enumDesc(file_google_protobuf_cpp_features, 0, 0);
var cpp = extDesc(file_google_protobuf_cpp_features, 0);

// node_modules/@bufbuild/protobuf/dist/esm/wkt/gen/google/protobuf/empty_pb.js
var file_google_protobuf_empty = fileDesc("Chtnb29nbGUvcHJvdG9idWYvZW1wdHkucHJvdG8SD2dvb2dsZS5wcm90b2J1ZiIHCgVFbXB0eUJ9ChNjb20uZ29vZ2xlLnByb3RvYnVmQgpFbXB0eVByb3RvUAFaLmdvb2dsZS5nb2xhbmcub3JnL3Byb3RvYnVmL3R5cGVzL2tub3duL2VtcHR5cGL4AQGiAgNHUEKqAh5Hb29nbGUuUHJvdG9idWYuV2VsbEtub3duVHlwZXNiBnByb3RvMw");
var EmptySchema = messageDesc(file_google_protobuf_empty, 0);

// node_modules/@bufbuild/protobuf/dist/esm/wkt/gen/google/protobuf/field_mask_pb.js
var file_google_protobuf_field_mask = fileDesc("CiBnb29nbGUvcHJvdG9idWYvZmllbGRfbWFzay5wcm90bxIPZ29vZ2xlLnByb3RvYnVmIhoKCUZpZWxkTWFzaxINCgVwYXRocxgBIAMoCUKFAQoTY29tLmdvb2dsZS5wcm90b2J1ZkIORmllbGRNYXNrUHJvdG9QAVoyZ29vZ2xlLmdvbGFuZy5vcmcvcHJvdG9idWYvdHlwZXMva25vd24vZmllbGRtYXNrcGL4AQGiAgNHUEKqAh5Hb29nbGUuUHJvdG9idWYuV2VsbEtub3duVHlwZXNiBnByb3RvMw");
var FieldMaskSchema = messageDesc(file_google_protobuf_field_mask, 0);

// node_modules/@bufbuild/protobuf/dist/esm/wkt/gen/google/protobuf/go_features_pb.js
var file_google_protobuf_go_features = fileDesc("CiFnb29nbGUvcHJvdG9idWYvZ29fZmVhdHVyZXMucHJvdG8SAnBiItEGCgpHb0ZlYXR1cmVzEqUBChpsZWdhY3lfdW5tYXJzaGFsX2pzb25fZW51bRgBIAEoCEKAAYgBAZgBBpgBAaIBCRIEdHJ1ZRiEB6IBChIFZmFsc2UY5weyAVsI6AcQ6AcaU1RoZSBsZWdhY3kgVW5tYXJzaGFsSlNPTiBBUEkgaXMgZGVwcmVjYXRlZCBhbmQgd2lsbCBiZSByZW1vdmVkIGluIGEgZnV0dXJlIGVkaXRpb24uEmoKCWFwaV9sZXZlbBgCIAEoDjIXLnBiLkdvRmVhdHVyZXMuQVBJTGV2ZWxCPogBAZgBA5gBAaIBGhIVQVBJX0xFVkVMX1VOU1BFQ0lGSUVEGIQHogEPEgpBUElfT1BBUVVFGOkHsgEDCOgHEmsKEXN0cmlwX2VudW1fcHJlZml4GAMgASgOMh4ucGIuR29GZWF0dXJlcy5TdHJpcEVudW1QcmVmaXhCMIgBAZgBBpgBB5gBAaIBGxIWU1RSSVBfRU5VTV9QUkVGSVhfS0VFUBiEB7IBAwjpBxJ4Cg1vcHRpbWl6ZV9tb2RlGAQgASgOMi8ucGIuR29GZWF0dXJlcy5PcHRpbWl6ZU1vZGVGZWF0dXJlLk9wdGltaXplTW9kZUIwiAEBmAEDmAEBogEeEhlPUFRJTUlaRV9NT0RFX1VOU1BFQ0lGSUVEGIQHsgEDCOkHGl4KE09wdGltaXplTW9kZUZlYXR1cmUiRwoMT3B0aW1pemVNb2RlEh0KGU9QVElNSVpFX01PREVfVU5TUEVDSUZJRUQQABIJCgVTUEVFRBABEg0KCUNPREVfU0laRRACIlMKCEFQSUxldmVsEhkKFUFQSV9MRVZFTF9VTlNQRUNJRklFRBAAEgwKCEFQSV9PUEVOEAESDgoKQVBJX0hZQlJJRBACEg4KCkFQSV9PUEFRVUUQAyKSAQoPU3RyaXBFbnVtUHJlZml4EiEKHVNUUklQX0VOVU1fUFJFRklYX1VOU1BFQ0lGSUVEEAASGgoWU1RSSVBfRU5VTV9QUkVGSVhfS0VFUBABEiMKH1NUUklQX0VOVU1fUFJFRklYX0dFTkVSQVRFX0JPVEgQAhIbChdTVFJJUF9FTlVNX1BSRUZJWF9TVFJJUBADOjwKAmdvEhsuZ29vZ2xlLnByb3RvYnVmLkZlYXR1cmVTZXQY6gcgASgLMg4ucGIuR29GZWF0dXJlc1ICZ29CL1otZ29vZ2xlLmdvbGFuZy5vcmcvcHJvdG9idWYvdHlwZXMvZ29mZWF0dXJlc3Bi", [file_google_protobuf_descriptor]);
var GoFeaturesSchema = messageDesc(file_google_protobuf_go_features, 0);
var GoFeatures_OptimizeModeFeatureSchema = messageDesc(file_google_protobuf_go_features, 0, 0);
var GoFeatures_OptimizeModeFeature_OptimizeMode;
(function(GoFeatures_OptimizeModeFeature_OptimizeMode2) {
  GoFeatures_OptimizeModeFeature_OptimizeMode2[GoFeatures_OptimizeModeFeature_OptimizeMode2["OPTIMIZE_MODE_UNSPECIFIED"] = 0] = "OPTIMIZE_MODE_UNSPECIFIED";
  GoFeatures_OptimizeModeFeature_OptimizeMode2[GoFeatures_OptimizeModeFeature_OptimizeMode2["SPEED"] = 1] = "SPEED";
  GoFeatures_OptimizeModeFeature_OptimizeMode2[GoFeatures_OptimizeModeFeature_OptimizeMode2["CODE_SIZE"] = 2] = "CODE_SIZE";
})(GoFeatures_OptimizeModeFeature_OptimizeMode || (GoFeatures_OptimizeModeFeature_OptimizeMode = {}));
var GoFeatures_OptimizeModeFeature_OptimizeModeSchema = enumDesc(file_google_protobuf_go_features, 0, 0, 0);
var GoFeatures_APILevel;
(function(GoFeatures_APILevel2) {
  GoFeatures_APILevel2[GoFeatures_APILevel2["API_LEVEL_UNSPECIFIED"] = 0] = "API_LEVEL_UNSPECIFIED";
  GoFeatures_APILevel2[GoFeatures_APILevel2["API_OPEN"] = 1] = "API_OPEN";
  GoFeatures_APILevel2[GoFeatures_APILevel2["API_HYBRID"] = 2] = "API_HYBRID";
  GoFeatures_APILevel2[GoFeatures_APILevel2["API_OPAQUE"] = 3] = "API_OPAQUE";
})(GoFeatures_APILevel || (GoFeatures_APILevel = {}));
var GoFeatures_APILevelSchema = enumDesc(file_google_protobuf_go_features, 0, 0);
var GoFeatures_StripEnumPrefix;
(function(GoFeatures_StripEnumPrefix2) {
  GoFeatures_StripEnumPrefix2[GoFeatures_StripEnumPrefix2["UNSPECIFIED"] = 0] = "UNSPECIFIED";
  GoFeatures_StripEnumPrefix2[GoFeatures_StripEnumPrefix2["KEEP"] = 1] = "KEEP";
  GoFeatures_StripEnumPrefix2[GoFeatures_StripEnumPrefix2["GENERATE_BOTH"] = 2] = "GENERATE_BOTH";
  GoFeatures_StripEnumPrefix2[GoFeatures_StripEnumPrefix2["STRIP"] = 3] = "STRIP";
})(GoFeatures_StripEnumPrefix || (GoFeatures_StripEnumPrefix = {}));
var GoFeatures_StripEnumPrefixSchema = enumDesc(file_google_protobuf_go_features, 0, 1);
var go = extDesc(file_google_protobuf_go_features, 0);

// node_modules/@bufbuild/protobuf/dist/esm/wkt/gen/google/protobuf/java_features_pb.js
var file_google_protobuf_java_features = fileDesc("CiNnb29nbGUvcHJvdG9idWYvamF2YV9mZWF0dXJlcy5wcm90bxICcGIigwgKDEphdmFGZWF0dXJlcxL+AQoSbGVnYWN5X2Nsb3NlZF9lbnVtGAEgASgIQuEBiAEBmAEEmAEBogEJEgR0cnVlGIQHogEKEgVmYWxzZRjnB7IBuwEI6AcQ6AcasgFUaGUgbGVnYWN5IGNsb3NlZCBlbnVtIGJlaGF2aW9yIGluIEphdmEgaXMgZGVwcmVjYXRlZCBhbmQgaXMgc2NoZWR1bGVkIHRvIGJlIHJlbW92ZWQgaW4gZWRpdGlvbiAyMDI1LiAgU2VlIGh0dHA6Ly9wcm90b2J1Zi5kZXYvcHJvZ3JhbW1pbmctZ3VpZGVzL2VudW0vI2phdmEgZm9yIG1vcmUgaW5mb3JtYXRpb24uEp8CCg91dGY4X3ZhbGlkYXRpb24YAiABKA4yHy5wYi5KYXZhRmVhdHVyZXMuVXRmOFZhbGlkYXRpb25C5AGIAQGYAQSYAQGiAQwSB0RFRkFVTFQYhAeyAcgBCOgHEOkHGr8BVGhlIEphdmEtc3BlY2lmaWMgdXRmOCB2YWxpZGF0aW9uIGZlYXR1cmUgaXMgZGVwcmVjYXRlZCBhbmQgaXMgc2NoZWR1bGVkIHRvIGJlIHJlbW92ZWQgaW4gZWRpdGlvbiAyMDI1LiAgVXRmOCB2YWxpZGF0aW9uIGJlaGF2aW9yIHNob3VsZCB1c2UgdGhlIGdsb2JhbCBjcm9zcy1sYW5ndWFnZSB1dGY4X3ZhbGlkYXRpb24gZmVhdHVyZS4SMAoKbGFyZ2VfZW51bRgDIAEoCEIciAEBmAEGmAEBogEKEgVmYWxzZRiEB7IBAwjpBxJRCh91c2Vfb2xkX291dGVyX2NsYXNzbmFtZV9kZWZhdWx0GAQgASgIQiiIAQGYAQGiAQkSBHRydWUYhAeiAQoSBWZhbHNlGOkHsgEGCOkHIOkHEn8KEm5lc3RfaW5fZmlsZV9jbGFzcxgFIAEoDjI3LnBiLkphdmFGZWF0dXJlcy5OZXN0SW5GaWxlQ2xhc3NGZWF0dXJlLk5lc3RJbkZpbGVDbGFzc0IqiAEBmAEDmAEGmAEIogELEgZMRUdBQ1kYhAeiAQcSAk5PGOkHsgEDCOkHGnwKFk5lc3RJbkZpbGVDbGFzc0ZlYXR1cmUiWAoPTmVzdEluRmlsZUNsYXNzEh4KGk5FU1RfSU5fRklMRV9DTEFTU19VTktOT1dOEAASBgoCTk8QARIHCgNZRVMQAhIUCgZMRUdBQ1kQAxoIIgYI6Qcg6QdKCAgBEICAgIACIkYKDlV0ZjhWYWxpZGF0aW9uEhsKF1VURjhfVkFMSURBVElPTl9VTktOT1dOEAASCwoHREVGQVVMVBABEgoKBlZFUklGWRACSgQIBhAHOkIKBGphdmESGy5nb29nbGUucHJvdG9idWYuRmVhdHVyZVNldBjpByABKAsyEC5wYi5KYXZhRmVhdHVyZXNSBGphdmFCKAoTY29tLmdvb2dsZS5wcm90b2J1ZkIRSmF2YUZlYXR1cmVzUHJvdG8", [file_google_protobuf_descriptor]);
var JavaFeaturesSchema = messageDesc(file_google_protobuf_java_features, 0);
var JavaFeatures_NestInFileClassFeatureSchema = messageDesc(file_google_protobuf_java_features, 0, 0);
var JavaFeatures_NestInFileClassFeature_NestInFileClass;
(function(JavaFeatures_NestInFileClassFeature_NestInFileClass2) {
  JavaFeatures_NestInFileClassFeature_NestInFileClass2[JavaFeatures_NestInFileClassFeature_NestInFileClass2["NEST_IN_FILE_CLASS_UNKNOWN"] = 0] = "NEST_IN_FILE_CLASS_UNKNOWN";
  JavaFeatures_NestInFileClassFeature_NestInFileClass2[JavaFeatures_NestInFileClassFeature_NestInFileClass2["NO"] = 1] = "NO";
  JavaFeatures_NestInFileClassFeature_NestInFileClass2[JavaFeatures_NestInFileClassFeature_NestInFileClass2["YES"] = 2] = "YES";
  JavaFeatures_NestInFileClassFeature_NestInFileClass2[JavaFeatures_NestInFileClassFeature_NestInFileClass2["LEGACY"] = 3] = "LEGACY";
})(JavaFeatures_NestInFileClassFeature_NestInFileClass || (JavaFeatures_NestInFileClassFeature_NestInFileClass = {}));
var JavaFeatures_NestInFileClassFeature_NestInFileClassSchema = enumDesc(file_google_protobuf_java_features, 0, 0, 0);
var JavaFeatures_Utf8Validation;
(function(JavaFeatures_Utf8Validation2) {
  JavaFeatures_Utf8Validation2[JavaFeatures_Utf8Validation2["UTF8_VALIDATION_UNKNOWN"] = 0] = "UTF8_VALIDATION_UNKNOWN";
  JavaFeatures_Utf8Validation2[JavaFeatures_Utf8Validation2["DEFAULT"] = 1] = "DEFAULT";
  JavaFeatures_Utf8Validation2[JavaFeatures_Utf8Validation2["VERIFY"] = 2] = "VERIFY";
})(JavaFeatures_Utf8Validation || (JavaFeatures_Utf8Validation = {}));
var JavaFeatures_Utf8ValidationSchema = enumDesc(file_google_protobuf_java_features, 0, 0);
var java = extDesc(file_google_protobuf_java_features, 0);

// node_modules/@bufbuild/protobuf/dist/esm/wkt/gen/google/protobuf/struct_pb.js
var file_google_protobuf_struct = fileDesc("Chxnb29nbGUvcHJvdG9idWYvc3RydWN0LnByb3RvEg9nb29nbGUucHJvdG9idWYihAEKBlN0cnVjdBIzCgZmaWVsZHMYASADKAsyIy5nb29nbGUucHJvdG9idWYuU3RydWN0LkZpZWxkc0VudHJ5GkUKC0ZpZWxkc0VudHJ5EgsKA2tleRgBIAEoCRIlCgV2YWx1ZRgCIAEoCzIWLmdvb2dsZS5wcm90b2J1Zi5WYWx1ZToCOAEi6gEKBVZhbHVlEjAKCm51bGxfdmFsdWUYASABKA4yGi5nb29nbGUucHJvdG9idWYuTnVsbFZhbHVlSAASFgoMbnVtYmVyX3ZhbHVlGAIgASgBSAASFgoMc3RyaW5nX3ZhbHVlGAMgASgJSAASFAoKYm9vbF92YWx1ZRgEIAEoCEgAEi8KDHN0cnVjdF92YWx1ZRgFIAEoCzIXLmdvb2dsZS5wcm90b2J1Zi5TdHJ1Y3RIABIwCgpsaXN0X3ZhbHVlGAYgASgLMhouZ29vZ2xlLnByb3RvYnVmLkxpc3RWYWx1ZUgAQgYKBGtpbmQiMwoJTGlzdFZhbHVlEiYKBnZhbHVlcxgBIAMoCzIWLmdvb2dsZS5wcm90b2J1Zi5WYWx1ZSobCglOdWxsVmFsdWUSDgoKTlVMTF9WQUxVRRAAQn8KE2NvbS5nb29nbGUucHJvdG9idWZCC1N0cnVjdFByb3RvUAFaL2dvb2dsZS5nb2xhbmcub3JnL3Byb3RvYnVmL3R5cGVzL2tub3duL3N0cnVjdHBi+AEBogIDR1BCqgIeR29vZ2xlLlByb3RvYnVmLldlbGxLbm93blR5cGVzYgZwcm90bzM");
var StructSchema = messageDesc(file_google_protobuf_struct, 0);
var ValueSchema = messageDesc(file_google_protobuf_struct, 1);
var ListValueSchema = messageDesc(file_google_protobuf_struct, 2);
var NullValue;
(function(NullValue2) {
  NullValue2[NullValue2["NULL_VALUE"] = 0] = "NULL_VALUE";
})(NullValue || (NullValue = {}));
var NullValueSchema = enumDesc(file_google_protobuf_struct, 0);

// node_modules/@bufbuild/protobuf/dist/esm/wkt/gen/google/protobuf/wrappers_pb.js
var file_google_protobuf_wrappers = fileDesc("Ch5nb29nbGUvcHJvdG9idWYvd3JhcHBlcnMucHJvdG8SD2dvb2dsZS5wcm90b2J1ZiIcCgtEb3VibGVWYWx1ZRINCgV2YWx1ZRgBIAEoASIbCgpGbG9hdFZhbHVlEg0KBXZhbHVlGAEgASgCIhsKCkludDY0VmFsdWUSDQoFdmFsdWUYASABKAMiHAoLVUludDY0VmFsdWUSDQoFdmFsdWUYASABKAQiGwoKSW50MzJWYWx1ZRINCgV2YWx1ZRgBIAEoBSIcCgtVSW50MzJWYWx1ZRINCgV2YWx1ZRgBIAEoDSIaCglCb29sVmFsdWUSDQoFdmFsdWUYASABKAgiHAoLU3RyaW5nVmFsdWUSDQoFdmFsdWUYASABKAkiGwoKQnl0ZXNWYWx1ZRINCgV2YWx1ZRgBIAEoDEKDAQoTY29tLmdvb2dsZS5wcm90b2J1ZkINV3JhcHBlcnNQcm90b1ABWjFnb29nbGUuZ29sYW5nLm9yZy9wcm90b2J1Zi90eXBlcy9rbm93bi93cmFwcGVyc3Bi+AEBogIDR1BCqgIeR29vZ2xlLlByb3RvYnVmLldlbGxLbm93blR5cGVzYgZwcm90bzM");
var DoubleValueSchema = messageDesc(file_google_protobuf_wrappers, 0);
var FloatValueSchema = messageDesc(file_google_protobuf_wrappers, 1);
var Int64ValueSchema = messageDesc(file_google_protobuf_wrappers, 2);
var UInt64ValueSchema = messageDesc(file_google_protobuf_wrappers, 3);
var Int32ValueSchema = messageDesc(file_google_protobuf_wrappers, 4);
var UInt32ValueSchema = messageDesc(file_google_protobuf_wrappers, 5);
var BoolValueSchema = messageDesc(file_google_protobuf_wrappers, 6);
var StringValueSchema = messageDesc(file_google_protobuf_wrappers, 7);
var BytesValueSchema = messageDesc(file_google_protobuf_wrappers, 8);

// node_modules/@bufbuild/protobuf/dist/esm/wkt/gen/google/protobuf/compiler/plugin_pb.js
var file_google_protobuf_compiler_plugin = fileDesc("CiVnb29nbGUvcHJvdG9idWYvY29tcGlsZXIvcGx1Z2luLnByb3RvEhhnb29nbGUucHJvdG9idWYuY29tcGlsZXIiRgoHVmVyc2lvbhINCgVtYWpvchgBIAEoBRINCgVtaW5vchgCIAEoBRINCgVwYXRjaBgDIAEoBRIOCgZzdWZmaXgYBCABKAkigQIKFENvZGVHZW5lcmF0b3JSZXF1ZXN0EhgKEGZpbGVfdG9fZ2VuZXJhdGUYASADKAkSEQoJcGFyYW1ldGVyGAIgASgJEjgKCnByb3RvX2ZpbGUYDyADKAsyJC5nb29nbGUucHJvdG9idWYuRmlsZURlc2NyaXB0b3JQcm90bxJFChdzb3VyY2VfZmlsZV9kZXNjcmlwdG9ycxgRIAMoCzIkLmdvb2dsZS5wcm90b2J1Zi5GaWxlRGVzY3JpcHRvclByb3RvEjsKEGNvbXBpbGVyX3ZlcnNpb24YAyABKAsyIS5nb29nbGUucHJvdG9idWYuY29tcGlsZXIuVmVyc2lvbiKSAwoVQ29kZUdlbmVyYXRvclJlc3BvbnNlEg0KBWVycm9yGAEgASgJEhoKEnN1cHBvcnRlZF9mZWF0dXJlcxgCIAEoBBIXCg9taW5pbXVtX2VkaXRpb24YAyABKAUSFwoPbWF4aW11bV9lZGl0aW9uGAQgASgFEkIKBGZpbGUYDyADKAsyNC5nb29nbGUucHJvdG9idWYuY29tcGlsZXIuQ29kZUdlbmVyYXRvclJlc3BvbnNlLkZpbGUafwoERmlsZRIMCgRuYW1lGAEgASgJEhcKD2luc2VydGlvbl9wb2ludBgCIAEoCRIPCgdjb250ZW50GA8gASgJEj8KE2dlbmVyYXRlZF9jb2RlX2luZm8YECABKAsyIi5nb29nbGUucHJvdG9idWYuR2VuZXJhdGVkQ29kZUluZm8iVwoHRmVhdHVyZRIQCgxGRUFUVVJFX05PTkUQABIbChdGRUFUVVJFX1BST1RPM19PUFRJT05BTBABEh0KGUZFQVRVUkVfU1VQUE9SVFNfRURJVElPTlMQAkJyChxjb20uZ29vZ2xlLnByb3RvYnVmLmNvbXBpbGVyQgxQbHVnaW5Qcm90b3NaKWdvb2dsZS5nb2xhbmcub3JnL3Byb3RvYnVmL3R5cGVzL3BsdWdpbnBiqgIYR29vZ2xlLlByb3RvYnVmLkNvbXBpbGVy", [file_google_protobuf_descriptor]);
var VersionSchema = messageDesc(file_google_protobuf_compiler_plugin, 0);
var CodeGeneratorRequestSchema = messageDesc(file_google_protobuf_compiler_plugin, 1);
var CodeGeneratorResponseSchema = messageDesc(file_google_protobuf_compiler_plugin, 2);
var CodeGeneratorResponse_FileSchema = messageDesc(file_google_protobuf_compiler_plugin, 2, 0);
var CodeGeneratorResponse_Feature;
(function(CodeGeneratorResponse_Feature2) {
  CodeGeneratorResponse_Feature2[CodeGeneratorResponse_Feature2["NONE"] = 0] = "NONE";
  CodeGeneratorResponse_Feature2[CodeGeneratorResponse_Feature2["PROTO3_OPTIONAL"] = 1] = "PROTO3_OPTIONAL";
  CodeGeneratorResponse_Feature2[CodeGeneratorResponse_Feature2["SUPPORTS_EDITIONS"] = 2] = "SUPPORTS_EDITIONS";
})(CodeGeneratorResponse_Feature || (CodeGeneratorResponse_Feature = {}));
var CodeGeneratorResponse_FeatureSchema = enumDesc(file_google_protobuf_compiler_plugin, 2, 0);

// node_modules/@bufbuild/protobuf/dist/esm/extensions.js
function getExtension(message, extension, options) {
  assertExtendee(extension, message);
  const ufs = filterUnknownFields(message.$unknown, extension);
  const [container, field, get] = createExtensionContainer(extension);
  const ctx = makeReadContext(options);
  for (const uf of ufs) {
    readField(container, new BinaryReader(uf.data), field, uf.wireType, ctx);
  }
  return get();
}
function setExtension(message, extension, value) {
  var _a;
  assertExtendee(extension, message);
  const ufs = ((_a = message.$unknown) !== null && _a !== void 0 ? _a : []).filter((uf) => uf.no !== extension.number);
  const [container, field] = createExtensionContainer(extension, value);
  const writer = new BinaryWriter();
  writeField(writer, { writeUnknownFields: true }, container, field);
  const reader = new BinaryReader(writer.finish());
  while (reader.pos < reader.len) {
    const [no, wireType] = reader.tag();
    const data = reader.skip(wireType, no);
    ufs.push({ no, wireType, data });
  }
  message.$unknown = ufs;
}
function clearExtension(message, extension) {
  assertExtendee(extension, message);
  if (message.$unknown === void 0) {
    return;
  }
  message.$unknown = message.$unknown.filter((uf) => uf.no !== extension.number);
}
function hasExtension(message, extension) {
  var _a;
  return extension.extendee.typeName === message.$typeName && !!((_a = message.$unknown) === null || _a === void 0 ? void 0 : _a.find((uf) => uf.no === extension.number));
}
function hasOption(element, option) {
  const message = element.proto.options;
  if (!message) {
    return false;
  }
  return hasExtension(message, option);
}
function getOption(element, option) {
  const message = element.proto.options;
  if (!message) {
    const [, , get] = createExtensionContainer(option);
    return get();
  }
  return getExtension(message, option);
}
function filterUnknownFields(unknownFields, extension) {
  if (unknownFields === void 0)
    return [];
  if (extension.fieldKind === "enum" || extension.fieldKind === "scalar") {
    for (let i = unknownFields.length - 1; i >= 0; --i) {
      if (unknownFields[i].no == extension.number) {
        return [unknownFields[i]];
      }
    }
    return [];
  }
  return unknownFields.filter((uf) => uf.no === extension.number);
}
function createExtensionContainer(extension, value) {
  const localName = extension.typeName;
  const field = Object.assign(Object.assign({}, extension), { kind: "field", parent: extension.extendee, localName });
  const desc = Object.assign(Object.assign({}, extension.extendee), { fields: [field], members: [field], oneofs: [] });
  const container = create(desc, value !== void 0 ? { [localName]: value } : void 0);
  return [
    reflect(desc, container),
    field,
    () => {
      const value2 = container[localName];
      if (value2 === void 0) {
        const desc2 = extension.message;
        if (isWrapperDesc(desc2)) {
          return scalarZeroValue(desc2.fields[0].scalar, desc2.fields[0].longAsString);
        }
        return create(desc2);
      }
      return value2;
    }
  ];
}
function assertExtendee(extension, message) {
  if (extension.extendee.typeName != message.$typeName) {
    throw new Error(`extension ${extension.typeName} can only be applied to message ${extension.extendee.typeName}`);
  }
}

// node_modules/@bufbuild/protobuf/dist/esm/equals.js
function equals(schema, a, b, options) {
  if (a.$typeName != schema.typeName || b.$typeName != schema.typeName) {
    return false;
  }
  if (a === b) {
    return true;
  }
  return reflectEquals(reflect(schema, a), reflect(schema, b), options);
}
function reflectEquals(a, b, opts) {
  if (a.desc.typeName === "google.protobuf.Any" && (opts === null || opts === void 0 ? void 0 : opts.unpackAny) == true) {
    return anyUnpackedEquals(a.message, b.message, opts);
  }
  for (const f of a.fields) {
    if (!fieldEquals(f, a, b, opts)) {
      return false;
    }
  }
  if ((opts === null || opts === void 0 ? void 0 : opts.unknown) == true && !unknownEquals(a, b, opts.registry)) {
    return false;
  }
  if ((opts === null || opts === void 0 ? void 0 : opts.extensions) == true && !extensionsEquals(a, b, opts)) {
    return false;
  }
  return true;
}
function fieldEquals(f, a, b, opts) {
  if (!a.isSet(f) && !b.isSet(f)) {
    return true;
  }
  if (!a.isSet(f) || !b.isSet(f)) {
    return false;
  }
  switch (f.fieldKind) {
    case "scalar":
      return scalarEquals(f.scalar, a.get(f), b.get(f));
    case "enum":
      return a.get(f) === b.get(f);
    case "message":
      return reflectEquals(a.get(f), b.get(f), opts);
    case "map": {
      const mapA = a.get(f);
      const mapB = b.get(f);
      const keys = [];
      for (const k of mapA.keys()) {
        if (!mapB.has(k)) {
          return false;
        }
        keys.push(k);
      }
      for (const k of mapB.keys()) {
        if (!mapA.has(k)) {
          return false;
        }
      }
      for (const key of keys) {
        const va = mapA.get(key);
        const vb = mapB.get(key);
        if (va === vb) {
          continue;
        }
        switch (f.mapKind) {
          case "enum":
            return false;
          case "message":
            if (!reflectEquals(va, vb, opts)) {
              return false;
            }
            break;
          case "scalar":
            if (!scalarEquals(f.scalar, va, vb)) {
              return false;
            }
            break;
        }
      }
      break;
    }
    case "list": {
      const listA = a.get(f);
      const listB = b.get(f);
      if (listA.size != listB.size) {
        return false;
      }
      for (let i = 0; i < listA.size; i++) {
        const va = listA.get(i);
        const vb = listB.get(i);
        if (va === vb) {
          continue;
        }
        switch (f.listKind) {
          case "enum":
            return false;
          case "message":
            if (!reflectEquals(va, vb, opts)) {
              return false;
            }
            break;
          case "scalar":
            if (!scalarEquals(f.scalar, va, vb)) {
              return false;
            }
            break;
        }
      }
      break;
    }
  }
  return true;
}
function anyUnpackedEquals(a, b, opts) {
  if (a.typeUrl !== b.typeUrl) {
    return false;
  }
  const unpackedA = anyUnpack(a, opts.registry);
  const unpackedB = anyUnpack(b, opts.registry);
  if (unpackedA && unpackedB) {
    const schema = opts.registry.getMessage(unpackedA.$typeName);
    if (schema) {
      return equals(schema, unpackedA, unpackedB, opts);
    }
  }
  return scalarEquals(ScalarType.BYTES, a.value, b.value);
}
function unknownEquals(a, b, registry) {
  function getTrulyUnknown(msg, registry2) {
    var _a;
    const u = (_a = msg.getUnknown()) !== null && _a !== void 0 ? _a : [];
    return registry2 ? u.filter((uf) => !registry2.getExtensionFor(msg.desc, uf.no)) : u;
  }
  const unknownA = getTrulyUnknown(a, registry);
  const unknownB = getTrulyUnknown(b, registry);
  if (unknownA.length != unknownB.length) {
    return false;
  }
  for (let i = 0; i < unknownA.length; i++) {
    const a2 = unknownA[i];
    const b2 = unknownB[i];
    if (a2.no != b2.no) {
      return false;
    }
    if (a2.wireType != b2.wireType) {
      return false;
    }
    if (!scalarEquals(ScalarType.BYTES, a2.data, b2.data)) {
      return false;
    }
  }
  return true;
}
function extensionsEquals(a, b, opts) {
  function getSetExtensions(msg, registry) {
    var _a;
    return ((_a = msg.getUnknown()) !== null && _a !== void 0 ? _a : []).map((uf) => registry.getExtensionFor(msg.desc, uf.no)).filter((e) => e != void 0).filter((e, index, arr) => arr.indexOf(e) === index);
  }
  const extensionsA = getSetExtensions(a, opts.registry);
  const extensionsB = getSetExtensions(b, opts.registry);
  if (extensionsA.length != extensionsB.length || extensionsA.some((e) => !extensionsB.includes(e))) {
    return false;
  }
  for (const extension of extensionsA) {
    const [containerA, field] = createExtensionContainer(extension, getExtension(a.message, extension));
    const [containerB] = createExtensionContainer(extension, getExtension(b.message, extension));
    if (!fieldEquals(field, containerA, containerB, opts)) {
      return false;
    }
  }
  return true;
}

// node_modules/@bufbuild/protobuf/dist/esm/wire/size-delimited.js
var defaultReadMaxBytes = 64 * 1024 * 1024;

// node_modules/@bufbuild/protobuf/dist/esm/to-json.js
var LEGACY_REQUIRED = 3;
var IMPLICIT = 2;
var jsonWriteDefaults = {
  alwaysEmitImplicit: false,
  enumAsInteger: false,
  useProtoFieldName: false
};
function makeWriteOptions(options) {
  return options ? Object.assign(Object.assign({}, jsonWriteDefaults), options) : jsonWriteDefaults;
}
function toJson(schema, message, options) {
  return reflectToJson(reflect(schema, message), makeWriteOptions(options));
}
function toJsonString(schema, message, options) {
  var _a;
  const jsonValue = toJson(schema, message, options);
  return JSON.stringify(jsonValue, null, (_a = options === null || options === void 0 ? void 0 : options.prettySpaces) !== null && _a !== void 0 ? _a : 0);
}
function enumToJson(descEnum, value) {
  var _a;
  if (descEnum.typeName == "google.protobuf.NullValue") {
    return null;
  }
  const name = (_a = descEnum.value[value]) === null || _a === void 0 ? void 0 : _a.name;
  if (name === void 0) {
    throw new Error(`${value} is not a value in ${descEnum}`);
  }
  return name;
}
function reflectToJson(msg, opts) {
  var _a;
  const wktJson = tryWktToJson(msg, opts);
  if (wktJson !== void 0)
    return wktJson;
  const json = {};
  for (const f of msg.sortedFields) {
    if (!msg.isSet(f)) {
      if (f.presence == LEGACY_REQUIRED) {
        throw new Error(`cannot encode ${f} to JSON: required field not set`);
      }
      if (!opts.alwaysEmitImplicit || f.presence !== IMPLICIT) {
        continue;
      }
    }
    const jsonValue = fieldToJson(f, msg.get(f), opts);
    if (jsonValue !== void 0) {
      json[jsonName(f, opts)] = jsonValue;
    }
  }
  if (opts.registry) {
    const tagSeen = /* @__PURE__ */ new Set();
    for (const { no } of (_a = msg.getUnknown()) !== null && _a !== void 0 ? _a : []) {
      if (!tagSeen.has(no)) {
        tagSeen.add(no);
        const extension = opts.registry.getExtensionFor(msg.desc, no);
        if (!extension) {
          continue;
        }
        const value = getExtension(msg.message, extension);
        const [container, field] = createExtensionContainer(extension, value);
        const jsonValue = fieldToJson(field, container.get(field), opts);
        if (jsonValue !== void 0) {
          json[extension.jsonName] = jsonValue;
        }
      }
    }
  }
  return json;
}
function fieldToJson(f, val, opts) {
  switch (f.fieldKind) {
    case "scalar":
      return scalarToJson(f, val);
    case "message":
      return reflectToJson(val, opts);
    case "enum":
      return enumToJsonInternal(f.enum, val, opts.enumAsInteger);
    case "list":
      return listToJson(val, opts);
    case "map":
      return mapToJson(val, opts);
  }
}
function mapToJson(map, opts) {
  const f = map.field();
  const jsonObj = {};
  switch (f.mapKind) {
    case "scalar":
      for (const [entryKey, entryValue] of map) {
        jsonObj[entryKey] = scalarToJson(f, entryValue);
      }
      break;
    case "message":
      for (const [entryKey, entryValue] of map) {
        jsonObj[entryKey] = reflectToJson(entryValue, opts);
      }
      break;
    case "enum":
      for (const [entryKey, entryValue] of map) {
        jsonObj[entryKey] = enumToJsonInternal(f.enum, entryValue, opts.enumAsInteger);
      }
      break;
  }
  return opts.alwaysEmitImplicit || map.size > 0 ? jsonObj : void 0;
}
function listToJson(list, opts) {
  const f = list.field();
  const jsonArr = [];
  switch (f.listKind) {
    case "scalar":
      for (const item of list) {
        jsonArr.push(scalarToJson(f, item));
      }
      break;
    case "enum":
      for (const item of list) {
        jsonArr.push(enumToJsonInternal(f.enum, item, opts.enumAsInteger));
      }
      break;
    case "message":
      for (const item of list) {
        jsonArr.push(reflectToJson(item, opts));
      }
      break;
  }
  return opts.alwaysEmitImplicit || jsonArr.length > 0 ? jsonArr : void 0;
}
function enumToJsonInternal(desc, value, enumAsInteger) {
  var _a;
  if (typeof value != "number") {
    throw new Error(`cannot encode ${desc} to JSON: expected number, got ${formatVal(value)}`);
  }
  if (desc.typeName == "google.protobuf.NullValue") {
    return null;
  }
  if (enumAsInteger) {
    return value;
  }
  const val = desc.value[value];
  return (_a = val === null || val === void 0 ? void 0 : val.name) !== null && _a !== void 0 ? _a : value;
}
function scalarToJson(field, value) {
  var _a, _b, _c, _d, _e, _f;
  switch (field.scalar) {
    // int32, fixed32, uint32: JSON value will be a decimal number. Either numbers or strings are accepted.
    case ScalarType.INT32:
    case ScalarType.SFIXED32:
    case ScalarType.SINT32:
    case ScalarType.FIXED32:
    case ScalarType.UINT32:
      if (typeof value != "number") {
        throw new Error(`cannot encode ${field} to JSON: ${(_a = checkField(field, value)) === null || _a === void 0 ? void 0 : _a.message}`);
      }
      return value;
    // float, double: JSON value will be a number or one of the special string values "NaN", "Infinity", and "-Infinity".
    // Either numbers or strings are accepted. Exponent notation is also accepted.
    case ScalarType.FLOAT:
    case ScalarType.DOUBLE:
      if (typeof value != "number") {
        throw new Error(`cannot encode ${field} to JSON: ${(_b = checkField(field, value)) === null || _b === void 0 ? void 0 : _b.message}`);
      }
      if (Number.isNaN(value))
        return "NaN";
      if (value === Number.POSITIVE_INFINITY)
        return "Infinity";
      if (value === Number.NEGATIVE_INFINITY)
        return "-Infinity";
      return value;
    // string:
    case ScalarType.STRING:
      if (typeof value != "string") {
        throw new Error(`cannot encode ${field} to JSON: ${(_c = checkField(field, value)) === null || _c === void 0 ? void 0 : _c.message}`);
      }
      return value;
    // bool:
    case ScalarType.BOOL:
      if (typeof value != "boolean") {
        throw new Error(`cannot encode ${field} to JSON: ${(_d = checkField(field, value)) === null || _d === void 0 ? void 0 : _d.message}`);
      }
      return value;
    // JSON value will be a decimal string. Either numbers or strings are accepted.
    case ScalarType.UINT64:
    case ScalarType.FIXED64:
    case ScalarType.INT64:
    case ScalarType.SFIXED64:
    case ScalarType.SINT64:
      if (typeof value == "bigint" || typeof value == "string" || typeof value == "number" && Number.isInteger(value)) {
        return value.toString();
      }
      throw new Error(`cannot encode ${field} to JSON: ${(_e = checkField(field, value)) === null || _e === void 0 ? void 0 : _e.message}`);
    // bytes: JSON value will be the data encoded as a string using standard base64 encoding with paddings.
    // Either standard or URL-safe base64 encoding with/without paddings are accepted.
    case ScalarType.BYTES:
      if (value instanceof Uint8Array) {
        return base64Encode(value);
      }
      throw new Error(`cannot encode ${field} to JSON: ${(_f = checkField(field, value)) === null || _f === void 0 ? void 0 : _f.message}`);
  }
}
function jsonName(f, opts) {
  return opts.useProtoFieldName ? f.name : f.jsonName;
}
function tryWktToJson(msg, opts) {
  if (!msg.desc.typeName.startsWith("google.protobuf.")) {
    return void 0;
  }
  switch (msg.desc.typeName) {
    case "google.protobuf.Any":
      return anyToJson(msg.message, opts);
    case "google.protobuf.Timestamp":
      return timestampToJson(msg.message);
    case "google.protobuf.Duration":
      return durationToJson(msg.message);
    case "google.protobuf.FieldMask":
      return fieldMaskToJson(msg.message);
    case "google.protobuf.Struct":
      return structToJson(msg.message);
    case "google.protobuf.Value":
      return valueToJson(msg.message);
    case "google.protobuf.ListValue":
      return listValueToJson(msg.message);
    default:
      if (isWrapperDesc(msg.desc)) {
        const valueField = msg.desc.fields[0];
        return scalarToJson(valueField, msg.get(valueField));
      }
      return void 0;
  }
}
function anyToJson(val, opts) {
  if (val.typeUrl === "") {
    return {};
  }
  const { registry } = opts;
  let message;
  let desc;
  if (registry) {
    message = anyUnpack(val, registry);
    if (message) {
      desc = registry.getMessage(message.$typeName);
    }
  }
  if (!desc || !message) {
    throw new Error(`cannot encode message ${val.$typeName} to JSON: "${val.typeUrl}" is not in the type registry`);
  }
  const reflected = reflect(desc, message);
  const json = hasCustomJsonRepresentation(desc) ? { value: tryWktToJson(reflected, opts) } : reflectToJson(reflected, opts);
  json["@type"] = val.typeUrl;
  return json;
}
function durationToJson(val) {
  const seconds = Number(val.seconds);
  const nanos = val.nanos;
  if (seconds > 315576e6 || seconds < -315576e6) {
    throw new Error(`cannot encode message ${val.$typeName} to JSON: value out of range`);
  }
  if (seconds > 0 && nanos < 0 || seconds < 0 && nanos > 0) {
    throw new Error(`cannot encode message ${val.$typeName} to JSON: nanos sign must match seconds sign`);
  }
  let text = val.seconds.toString();
  if (nanos !== 0) {
    let nanosStr = Math.abs(nanos).toString();
    nanosStr = "0".repeat(9 - nanosStr.length) + nanosStr;
    if (nanosStr.substring(3) === "000000") {
      nanosStr = nanosStr.substring(0, 3);
    } else if (nanosStr.substring(6) === "000") {
      nanosStr = nanosStr.substring(0, 6);
    }
    text += "." + nanosStr;
    if (nanos < 0 && seconds == 0) {
      text = "-" + text;
    }
  }
  return text + "s";
}
function fieldMaskToJson(val) {
  return val.paths.map((p) => {
    if (protoSnakeCase(protoCamelCase(p)) !== p) {
      throw new Error(`cannot encode message ${val.$typeName} to JSON: lowerCamelCase of path name "${p}" is irreversible`);
    }
    return protoCamelCase(p);
  }).join(",");
}
function structToJson(val) {
  const json = {};
  for (const [k, v] of Object.entries(val.fields)) {
    json[k] = valueToJson(v);
  }
  return json;
}
function valueToJson(val) {
  switch (val.kind.case) {
    case "nullValue":
      return null;
    case "numberValue":
      if (!Number.isFinite(val.kind.value)) {
        throw new Error(`${val.$typeName} cannot be NaN or Infinity`);
      }
      return val.kind.value;
    case "boolValue":
      return val.kind.value;
    case "stringValue":
      return val.kind.value;
    case "structValue":
      return structToJson(val.kind.value);
    case "listValue":
      return listValueToJson(val.kind.value);
    default:
      throw new Error(`${val.$typeName} must have a value`);
  }
}
function listValueToJson(val) {
  return val.values.map(valueToJson);
}
function timestampToJson(val) {
  const ms = Number(val.seconds) * 1e3;
  if (ms < Date.parse("0001-01-01T00:00:00Z") || ms > Date.parse("9999-12-31T23:59:59Z")) {
    throw new Error(`cannot encode message ${val.$typeName} to JSON: must be from 0001-01-01T00:00:00Z to 9999-12-31T23:59:59Z inclusive`);
  }
  if (val.nanos < 0) {
    throw new Error(`cannot encode message ${val.$typeName} to JSON: nanos must not be negative`);
  }
  if (val.nanos > 999999999) {
    throw new Error(`cannot encode message ${val.$typeName} to JSON: nanos must not be greater than 99999999`);
  }
  let z = "Z";
  if (val.nanos > 0) {
    const nanosStr = (val.nanos + 1e9).toString().substring(1);
    if (nanosStr.substring(3) === "000000") {
      z = "." + nanosStr.substring(0, 3) + "Z";
    } else if (nanosStr.substring(6) === "000") {
      z = "." + nanosStr.substring(0, 6) + "Z";
    } else {
      z = "." + nanosStr + "Z";
    }
  }
  return new Date(ms).toISOString().replace(".000Z", z);
}

// node_modules/@bufbuild/protobuf/dist/esm/from-json.js
function makeReadContext2(options) {
  return Object.assign(Object.assign({ ignoreUnknownFields: false, recursionLimit: 100 }, options), { depth: 0 });
}
function fromJsonString(schema, json, options) {
  return fromJson(schema, parseJsonString(json, schema.typeName), options);
}
function mergeFromJsonString(schema, target, json, options) {
  return mergeFromJson(schema, target, parseJsonString(json, schema.typeName), options);
}
function fromJson(schema, json, options) {
  const msg = reflect(schema);
  try {
    readMessage(msg, json, makeReadContext2(options));
  } catch (e) {
    if (isFieldError(e)) {
      throw new Error(`cannot decode ${e.field()} from JSON: ${e.message}`, {
        cause: e
      });
    }
    throw e;
  }
  return msg.message;
}
function mergeFromJson(schema, target, json, options) {
  try {
    readMessage(reflect(schema, target), json, makeReadContext2(options));
  } catch (e) {
    if (isFieldError(e)) {
      throw new Error(`cannot decode ${e.field()} from JSON: ${e.message}`, {
        cause: e
      });
    }
    throw e;
  }
  return target;
}
function enumFromJson(descEnum, json) {
  return readEnum(descEnum, json, false);
}
function isEnumJson(descEnum, value) {
  return void 0 !== descEnum.values.find((v) => v.name === value);
}
var messageJsonFields = /* @__PURE__ */ new WeakMap();
function getJsonField(desc, jsonKey) {
  var _a;
  if (!messageJsonFields.has(desc)) {
    const jsonNames = /* @__PURE__ */ new Map();
    for (const field of desc.fields) {
      jsonNames.set(field.name, field).set(field.jsonName, field);
    }
    messageJsonFields.set(desc, jsonNames);
  }
  return (_a = messageJsonFields.get(desc)) === null || _a === void 0 ? void 0 : _a.get(jsonKey);
}
function readMessage(msg, json, ctx) {
  var _a;
  if (++ctx.depth > ctx.recursionLimit) {
    throw new Error(`cannot decode ${msg.desc} from JSON: maximum recursion depth of ${ctx.recursionLimit} reached`);
  }
  if (tryWktFromJson(msg, json, ctx)) {
    ctx.depth--;
    return;
  }
  if (json == null || Array.isArray(json) || typeof json != "object") {
    throw new Error(`cannot decode ${msg.desc} from JSON: ${formatVal(json)}`);
  }
  const oneofSeen = /* @__PURE__ */ new Map();
  const fieldSeen = /* @__PURE__ */ new Set();
  for (const [jsonKey, jsonValue] of Object.entries(json)) {
    const field = getJsonField(msg.desc, jsonKey);
    if (field) {
      if (fieldSeen.has(field)) {
        throw new FieldError(field, "set multiple times");
      }
      fieldSeen.add(field);
      if (field.oneof && jsonValue === null && field.fieldKind == "scalar") {
        continue;
      }
      if (field.oneof) {
        const seen = oneofSeen.get(field.oneof);
        if (seen !== void 0) {
          throw new FieldError(field.oneof, `oneof set multiple times by ${seen.name} and ${field.name}`);
        }
        oneofSeen.set(field.oneof, field);
      }
      readField2(msg, field, jsonValue, ctx);
    } else {
      let extension = void 0;
      if (jsonKey.startsWith("[") && jsonKey.endsWith("]") && // biome-ignore lint/suspicious/noAssignInExpressions: no
      (extension = (_a = ctx.registry) === null || _a === void 0 ? void 0 : _a.getExtension(jsonKey.substring(1, jsonKey.length - 1))) && extension.extendee.typeName === msg.desc.typeName) {
        const [container, field2, get] = createExtensionContainer(extension);
        readField2(container, field2, jsonValue, ctx);
        setExtension(msg.message, extension, get());
      }
      if (!extension && !ctx.ignoreUnknownFields) {
        throw new Error(`cannot decode ${msg.desc} from JSON: key "${jsonKey}" is unknown`);
      }
    }
  }
  ctx.depth--;
}
function readField2(msg, field, json, ctx) {
  switch (field.fieldKind) {
    case "scalar":
      readScalarField(msg, field, json);
      break;
    case "enum":
      readEnumField(msg, field, json, ctx);
      break;
    case "message":
      readMessageField(msg, field, json, ctx);
      break;
    case "list":
      readListField(msg.get(field), json, ctx);
      break;
    case "map":
      readMapField(msg.get(field), json, ctx);
      break;
  }
}
function readListOrMapItem(field, json, ctx) {
  if (field.scalar && json !== null) {
    return scalarFromJson(field, json);
  }
  if (field.message && !isResetSentinelNullValue(field, json)) {
    const msgValue = reflect(field.message);
    readMessage(msgValue, json, ctx);
    return msgValue;
  }
  if (field.enum && !isResetSentinelNullValue(field, json)) {
    return readEnum(field.enum, json, ctx.ignoreUnknownFields);
  }
  throw new FieldError(field, `${field.fieldKind === "list" ? "list item" : "map value"} must not be null`);
}
function readMapField(map, json, ctx) {
  if (json === null) {
    return;
  }
  const field = map.field();
  if (typeof json != "object" || Array.isArray(json)) {
    throw new FieldError(field, "expected object, got " + formatVal(json));
  }
  const seen = /* @__PURE__ */ new Set();
  for (const [jsonMapKey, jsonMapValue] of Object.entries(json)) {
    const key = mapKeyFromJson(field.mapKey, jsonMapKey);
    if (seen.has(key)) {
      throw new FieldError(field, `duplicate map key "${jsonMapKey}"`);
    }
    seen.add(key);
    const value = readListOrMapItem(field, jsonMapValue, ctx);
    if (value !== tokenIgnoredUnknownEnum) {
      map.set(key, value);
    }
  }
}
function readListField(list, json, ctx) {
  if (json === null) {
    return;
  }
  const field = list.field();
  if (!Array.isArray(json)) {
    throw new FieldError(field, "expected Array, got " + formatVal(json));
  }
  for (const jsonItem of json) {
    const value = readListOrMapItem(field, jsonItem, ctx);
    if (value !== tokenIgnoredUnknownEnum) {
      list.add(value);
    }
  }
}
function readMessageField(msg, field, json, ctx) {
  if (isResetSentinelNullValue(field, json)) {
    msg.clear(field);
    return;
  }
  const msgValue = msg.isSet(field) ? msg.get(field) : reflect(field.message);
  readMessage(msgValue, json, ctx);
  msg.set(field, msgValue);
}
function readEnumField(msg, field, json, ctx) {
  if (isResetSentinelNullValue(field, json)) {
    msg.clear(field);
    return;
  }
  const enumValue = readEnum(field.enum, json, ctx.ignoreUnknownFields);
  if (enumValue !== tokenIgnoredUnknownEnum) {
    msg.set(field, enumValue);
  }
}
function readScalarField(msg, field, json) {
  if (json === null) {
    msg.clear(field);
  } else {
    msg.set(field, scalarFromJson(field, json));
  }
}
function isResetSentinelNullValue(field, json) {
  var _a, _b;
  return json === null && ((_a = field.message) === null || _a === void 0 ? void 0 : _a.typeName) != "google.protobuf.Value" && ((_b = field.enum) === null || _b === void 0 ? void 0 : _b.typeName) != "google.protobuf.NullValue";
}
var tokenIgnoredUnknownEnum = Symbol();
function readEnum(desc, json, ignoreUnknownFields) {
  if (json === null) {
    return desc.values[0].number;
  }
  switch (typeof json) {
    case "number":
      if (Number.isInteger(json)) {
        return json;
      }
      break;
    case "string":
      const value = desc.values.find((ev) => ev.name === json);
      if (value !== void 0) {
        return value.number;
      }
      if (ignoreUnknownFields) {
        return tokenIgnoredUnknownEnum;
      }
      break;
  }
  throw new Error(`cannot decode ${desc} from JSON: ${formatVal(json)}`);
}
function scalarFromJson(field, json) {
  switch (field.scalar) {
    // float, double: JSON value will be a number or one of the special string values "NaN", "Infinity", and "-Infinity".
    // Either numbers or strings are accepted. Exponent notation is also accepted.
    case ScalarType.DOUBLE:
    case ScalarType.FLOAT:
      if (json === "NaN")
        return NaN;
      if (json === "Infinity")
        return Number.POSITIVE_INFINITY;
      if (json === "-Infinity")
        return Number.NEGATIVE_INFINITY;
      if (typeof json == "number") {
        if (Number.isNaN(json)) {
          throw new FieldError(field, "unexpected NaN number");
        }
        if (!Number.isFinite(json)) {
          throw new FieldError(field, "unexpected infinite number");
        }
        break;
      }
      if (typeof json == "string") {
        if (json === "") {
          break;
        }
        if (json.trim().length !== json.length) {
          break;
        }
        const float = Number(json);
        if (!Number.isFinite(float)) {
          break;
        }
        return float;
      }
      break;
    // int32, fixed32, uint32: JSON value will be a decimal number. Either numbers or strings are accepted.
    case ScalarType.INT32:
    case ScalarType.FIXED32:
    case ScalarType.SFIXED32:
    case ScalarType.SINT32:
    case ScalarType.UINT32:
      return int32FromJson(json);
    // bytes: JSON value will be the data encoded as a string using standard base64 encoding with paddings.
    // Either standard or URL-safe base64 encoding with/without paddings are accepted.
    case ScalarType.BYTES:
      if (typeof json == "string") {
        if (json === "") {
          return new Uint8Array(0);
        }
        try {
          return base64Decode(json);
        } catch (e) {
          const message = e instanceof Error ? e.message : String(e);
          throw new FieldError(field, message);
        }
      }
      break;
  }
  return json;
}
function mapKeyFromJson(type, jsonString) {
  switch (type) {
    case ScalarType.BOOL:
      switch (jsonString) {
        case "true":
          return true;
        case "false":
          return false;
      }
      return jsonString;
    case ScalarType.INT32:
    case ScalarType.FIXED32:
    case ScalarType.UINT32:
    case ScalarType.SFIXED32:
    case ScalarType.SINT32:
      return int32FromJson(jsonString);
    case ScalarType.INT64:
    case ScalarType.SINT64:
    case ScalarType.SFIXED64:
    case ScalarType.UINT64:
    case ScalarType.FIXED64:
      return /^-?0+$/.test(jsonString) ? "0" : jsonString.replace(/^(-?)0+(?=\d)/, "$1");
    default:
      return jsonString;
  }
}
function int32FromJson(json) {
  if (typeof json == "string") {
    if (json === "") {
      return json;
    }
    if (json.trim().length !== json.length) {
      return json;
    }
    const num = Number(json);
    if (Number.isNaN(num)) {
      return json;
    }
    return num;
  }
  return json;
}
function parseJsonString(jsonString, typeName) {
  let json;
  try {
    json = JSON.parse(jsonString);
  } catch (e) {
    const message = e instanceof Error ? e.message : String(e);
    throw new Error(
      `cannot decode message ${typeName} from JSON: ${message}`,
      // @ts-expect-error we use the ES2022 error CTOR option "cause" for better stack traces
      { cause: e }
    );
  }
  checkDuplicateKeys(jsonString, typeName);
  return json;
}
function checkDuplicateKeys(jsonString, typeName) {
  const stack = [];
  let expectKey = false;
  let i = 0;
  while (i < jsonString.length) {
    switch (jsonString[i]) {
      case "{":
        stack.push(/* @__PURE__ */ new Set());
        expectKey = true;
        i++;
        break;
      case "[":
        stack.push(null);
        expectKey = false;
        i++;
        break;
      case "}":
      case "]":
        stack.pop();
        expectKey = false;
        i++;
        break;
      case ",":
        expectKey = stack[stack.length - 1] != null;
        i++;
        break;
      case ":":
        expectKey = false;
        i++;
        break;
      case '"': {
        const open = i++;
        let escaped = false;
        while (i < jsonString.length) {
          if (jsonString[i] == "\\") {
            escaped = true;
            i += 2;
            continue;
          }
          if (jsonString[i] == '"') {
            break;
          }
          i++;
        }
        const close = i++;
        const seen = stack[stack.length - 1];
        if (expectKey && seen) {
          const name = escaped ? JSON.parse(jsonString.substring(open, close + 1)) : jsonString.substring(open + 1, close);
          if (seen.has(name)) {
            throw new Error(`cannot decode message ${typeName} from JSON: duplicate object key "${name}"`);
          }
          seen.add(name);
        }
        expectKey = false;
        break;
      }
      default:
        i++;
        break;
    }
  }
}
function tryWktFromJson(msg, jsonValue, ctx) {
  if (!msg.desc.typeName.startsWith("google.protobuf.")) {
    return false;
  }
  switch (msg.desc.typeName) {
    case "google.protobuf.Any":
      anyFromJson(msg.message, jsonValue, ctx);
      return true;
    case "google.protobuf.Timestamp":
      timestampFromJson(msg.message, jsonValue);
      return true;
    case "google.protobuf.Duration":
      durationFromJson(msg.message, jsonValue);
      return true;
    case "google.protobuf.FieldMask":
      fieldMaskFromJson(msg.message, jsonValue);
      return true;
    case "google.protobuf.Struct":
      structFromJson(msg.message, jsonValue, ctx);
      return true;
    case "google.protobuf.Value":
      valueFromJson(msg.message, jsonValue, ctx);
      return true;
    case "google.protobuf.ListValue":
      listValueFromJson(msg.message, jsonValue, ctx);
      return true;
    default:
      if (isWrapperDesc(msg.desc)) {
        const valueField = msg.desc.fields[0];
        if (jsonValue === null) {
          msg.clear(valueField);
        } else {
          msg.set(valueField, scalarFromJson(valueField, jsonValue));
        }
        return true;
      }
      return false;
  }
}
function anyFromJson(any, json, ctx) {
  var _a;
  if (json === null || Array.isArray(json) || typeof json != "object") {
    throw new Error(`cannot decode message ${any.$typeName} from JSON: expected object but got ${formatVal(json)}`);
  }
  if (Object.keys(json).length == 0) {
    return;
  }
  const typeUrl = json["@type"];
  if (typeof typeUrl != "string" || typeUrl == "") {
    throw new Error(`cannot decode message ${any.$typeName} from JSON: "@type" is empty`);
  }
  const typeName = typeUrl.includes("/") ? typeUrl.substring(typeUrl.lastIndexOf("/") + 1) : typeUrl;
  if (!typeName.length) {
    throw new Error(`cannot decode message ${any.$typeName} from JSON: "@type" is invalid`);
  }
  const desc = (_a = ctx.registry) === null || _a === void 0 ? void 0 : _a.getMessage(typeName);
  if (!desc) {
    throw new Error(`cannot decode message ${any.$typeName} from JSON: ${typeUrl} is not in the type registry`);
  }
  const msg = reflect(desc);
  if (hasCustomJsonRepresentation(desc) && Object.prototype.hasOwnProperty.call(json, "value")) {
    const value = json.value;
    readMessage(msg, value, ctx);
  } else {
    const copy = Object.assign({}, json);
    delete copy["@type"];
    readMessage(msg, copy, ctx);
  }
  anyPack(msg.desc, msg.message, any);
}
function timestampFromJson(timestamp, json) {
  if (typeof json !== "string") {
    throw new Error(`cannot decode message ${timestamp.$typeName} from JSON: ${formatVal(json)}`);
  }
  const matches = json.match(/^([0-9]{4})-([0-9]{2})-([0-9]{2})T([0-9]{2}):([0-9]{2}):([0-9]{2})(?:\.([0-9]{1,9}))?(?:Z|([+-][0-9][0-9]:[0-9][0-9]))$/);
  if (!matches) {
    throw new Error(`cannot decode message ${timestamp.$typeName} from JSON: invalid RFC 3339 string`);
  }
  const ms = Date.parse(
    // biome-ignore format: want this to read well
    matches[1] + "-" + matches[2] + "-" + matches[3] + "T" + matches[4] + ":" + matches[5] + ":" + matches[6] + (matches[8] ? matches[8] : "Z")
  );
  if (Number.isNaN(ms)) {
    throw new Error(`cannot decode message ${timestamp.$typeName} from JSON: invalid RFC 3339 string`);
  }
  if (ms < Date.parse("0001-01-01T00:00:00Z") || ms > Date.parse("9999-12-31T23:59:59Z")) {
    throw new Error(`cannot decode message ${timestamp.$typeName} from JSON: must be from 0001-01-01T00:00:00Z to 9999-12-31T23:59:59Z inclusive`);
  }
  timestamp.seconds = protoInt64.parse(ms / 1e3);
  timestamp.nanos = 0;
  if (matches[7]) {
    timestamp.nanos = parseInt("1" + matches[7] + "0".repeat(9 - matches[7].length)) - 1e9;
  }
}
function durationFromJson(duration, json) {
  if (typeof json !== "string") {
    throw new Error(`cannot decode message ${duration.$typeName} from JSON: ${formatVal(json)}`);
  }
  const match = json.match(/^(-?[0-9]+)(?:\.([0-9]+))?s/);
  if (match === null) {
    throw new Error(`cannot decode message ${duration.$typeName} from JSON: ${formatVal(json)}`);
  }
  const longSeconds = Number(match[1]);
  if (longSeconds > 315576e6 || longSeconds < -315576e6) {
    throw new Error(`cannot decode message ${duration.$typeName} from JSON: ${formatVal(json)}`);
  }
  duration.seconds = protoInt64.parse(longSeconds);
  if (typeof match[2] !== "string") {
    return;
  }
  const nanosStr = match[2] + "0".repeat(9 - match[2].length);
  duration.nanos = parseInt(nanosStr);
  if (longSeconds < 0 || Object.is(longSeconds, -0)) {
    duration.nanos = -duration.nanos;
  }
}
function fieldMaskFromJson(fieldMask, json) {
  if (typeof json !== "string") {
    throw new Error(`cannot decode message ${fieldMask.$typeName} from JSON: ${formatVal(json)}`);
  }
  if (json === "") {
    return;
  }
  fieldMask.paths = json.split(",").map((path) => {
    if (path.includes("_")) {
      throw new Error(`cannot decode message ${fieldMask.$typeName} from JSON: path names must be lowerCamelCase`);
    }
    return protoSnakeCase(path);
  });
}
function structFromJson(struct, json, ctx) {
  if (typeof json != "object" || json == null || Array.isArray(json)) {
    throw new Error(`cannot decode message ${struct.$typeName} from JSON ${formatVal(json)}`);
  }
  for (const [k, v] of Object.entries(json)) {
    const parsedV = create(ValueSchema);
    valueFromJson(parsedV, v, ctx);
    struct.fields[k] = parsedV;
  }
}
function valueFromJson(value, json, ctx) {
  if (++ctx.depth > ctx.recursionLimit) {
    throw new Error(`cannot decode ${value.$typeName} from JSON: maximum recursion depth of ${ctx.recursionLimit} reached`);
  }
  switch (typeof json) {
    case "number":
      value.kind = { case: "numberValue", value: json };
      break;
    case "string":
      value.kind = { case: "stringValue", value: json };
      break;
    case "boolean":
      value.kind = { case: "boolValue", value: json };
      break;
    case "object":
      if (json === null) {
        value.kind = { case: "nullValue", value: NullValue.NULL_VALUE };
      } else if (Array.isArray(json)) {
        const listValue = create(ListValueSchema);
        listValueFromJson(listValue, json, ctx);
        value.kind = { case: "listValue", value: listValue };
      } else {
        const struct = create(StructSchema);
        structFromJson(struct, json, ctx);
        value.kind = { case: "structValue", value: struct };
      }
      break;
    default:
      throw new Error(`cannot decode message ${value.$typeName} from JSON ${formatVal(json)}`);
  }
  ctx.depth--;
  return value;
}
function listValueFromJson(listValue, json, ctx) {
  if (!Array.isArray(json)) {
    throw new Error(`cannot decode message ${listValue.$typeName} from JSON ${formatVal(json)}`);
  }
  for (const e of json) {
    const value = create(ValueSchema);
    valueFromJson(value, e, ctx);
    listValue.values.push(value);
  }
}

// node_modules/@bufbuild/protobuf/dist/esm/merge.js
function merge(schema, target, source) {
  reflectMerge(reflect(schema, target), reflect(schema, source));
}
function reflectMerge(target, source) {
  var _a;
  var _b;
  const sourceUnknown = source.message.$unknown;
  if (sourceUnknown !== void 0 && sourceUnknown.length > 0) {
    (_a = (_b = target.message).$unknown) !== null && _a !== void 0 ? _a : _b.$unknown = [];
    target.message.$unknown.push(...sourceUnknown);
  }
  for (const f of target.fields) {
    if (!source.isSet(f)) {
      continue;
    }
    switch (f.fieldKind) {
      case "scalar":
      case "enum":
        target.set(f, source.get(f));
        break;
      case "message":
        if (target.isSet(f)) {
          reflectMerge(target.get(f), source.get(f));
        } else {
          target.set(f, source.get(f));
        }
        break;
      case "list":
        const list = target.get(f);
        for (const e of source.get(f)) {
          list.add(e);
        }
        break;
      case "map":
        const map = target.get(f);
        for (const [k, v] of source.get(f)) {
          map.set(k, v);
        }
        break;
    }
  }
}

// node_modules/@bufbuild/protobuf/dist/esm/unknown-enum.js
function isUnknownEnum(desc, value) {
  return desc.value[value] === void 0;
}

export {
  file_google_protobuf_any,
  anyPack,
  getExtension,
  setExtension,
  clearExtension,
  hasExtension,
  hasOption,
  getOption,
  equals,
  toJson,
  toJsonString,
  enumToJson,
  fromJsonString,
  mergeFromJsonString,
  fromJson,
  mergeFromJson,
  enumFromJson,
  isEnumJson,
  merge,
  isUnknownEnum
};
//# sourceMappingURL=chunk-RQE5CLH2.js.map
