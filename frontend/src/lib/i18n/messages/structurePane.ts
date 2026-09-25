import type { AreaMessages } from '../types'

// source: frontend/src/components/StructurePane.tsx
//
// Written by `node scripts/i18n.mjs extract`. The first column is the text the
// code was written with and the other two are its translations; a correction
// belongs here. The marker line above is read back by the script, so it stays.

export const structurePane = {
  'structurePane.copied': ['{what} copied', '{what} 已复制', '{what} 已複製'],
  'structurePane.create-table': ['Create table {name}?', '创建表 {name}？', '建立資料表 {name}？'],
  'structurePane.apply-changes-to-this-table': [
    'Apply changes to this table?',
    '应用对该表的更改？',
    '套用對此資料表的變更？',
  ],
  'structurePane.create': ['Create', '创建', '建立'],
  'structurePane.apply': ['Apply', '应用', '套用'],
  'structurePane.this-engine-cannot-do-everything-the-design-asks': [
    'This engine cannot do everything the design asks for',
    '该引擎无法完全满足设计的要求',
    '此引擎無法完全滿足設計的要求',
  ],
  'structurePane.the-statements-run-one-at-a-time-in-this-order': [
    'The statements run one at a time, in this order.',
    '语句将按以下顺序逐条执行。',
    '語句將依下列順序逐條執行。',
  ],
  'structurePane.statement-n-of-m-failed': [
    'Statement {index} of {total} failed: {error}',
    '第 {index} / {total} 条语句失败：{error}',
    '第 {index} / {total} 條語句失敗：{error}',
  ],
  'structurePane.applied-of': [
    'Applied {executed} of {n} statement|Applied {executed} of {n} statements',
    '已执行 {executed} / {n} 条语句|已执行 {executed} / {n} 条语句',
    '已執行 {executed} / {n} 條語句|已執行 {executed} / {n} 條語句',
  ],
  'structurePane.table-created': ['Table {name} created', '表 {name} 已创建', '資料表 {name} 已建立'],
  'structurePane.table-updated': [
    'Table {object} updated ({n} statement)|Table {object} updated ({n} statements)',
    '表 {object} 已更新（{n} 条语句）|表 {object} 已更新（{n} 条语句）',
    '資料表 {object} 已更新（{n} 條語句）|資料表 {object} 已更新（{n} 條語句）',
  ],
  'structurePane.could-not-read-the-structure': [
    'Could not read the structure',
    '无法读取结构',
    '無法讀取結構',
  ],
  'structurePane.retry': ['Retry', '重试', '重試'],
  'structurePane.no-structure-available': ['No structure available', '没有可用的结构', '沒有可用的結構'],
  'structurePane.no-indexes': ['No indexes', '无索引', '無索引'],
  'structurePane.no-foreign-keys': ['No foreign keys', '无外键', '無外鍵'],
  'structurePane.definition': ['Definition', '定义', '定義'],
  'structurePane.copy-ddl': ['Copy DDL', '复制 DDL', '複製 DDL'],
  'structurePane.copy-the-definition-script': ['Copy the definition script', '复制定义脚本', '複製定義指令碼'],
  'structurePane.copy': ['Copy', '复制', '複製'],
  'structurePane.open-this-definition-in-the-ddl-editor-where-it': [
    'Open this definition in the DDL editor, where it can be changed and run',
    '在 DDL 编辑器中打开该定义，可在此修改并执行',
    '在 DDL 編輯器中開啟此定義，可在此修改並執行',
  ],
  'structurePane.edit-in-ddl-editor': ['Edit in DDL editor', '在 DDL 编辑器中编辑', '在 DDL 編輯器中編輯'],
  'structurePane.save': ['Save', '保存', '儲存'],
  'structurePane.ddl-reconstructed-from-catalog-metadata': [
    'DDL reconstructed from catalog metadata.',
    'DDL 由目录元数据重建。',
    'DDL 由目錄中繼資料重建。',
  ],
  'structurePane.definition-script-reconstructed-from-the-sampled': [
    'Definition script reconstructed from the sampled documents and the index list.',
    '定义脚本由抽样文档和索引列表重建。',
    '定義指令碼由取樣文件與索引清單重建。',
  ],
  'structurePane.table-name': ['Table name', '表名', '資料表名稱'],
  'structurePane.add-a-field-at-the-end-of-the-list': [
    'Add a field at the end of the list',
    '在列表末尾添加字段',
    '在清單末端新增欄位',
  ],
  'structurePane.add-field': ['Add field', '添加字段', '新增欄位'],
  'structurePane.drop-the-selected-field-and-its-data': [
    'Drop the selected field and its data',
    '删除所选字段及其数据',
    '刪除所選欄位及其資料',
  ],
  'structurePane.drop-field': ['Drop field {name}?', '删除字段 {name}？', '刪除欄位 {name}？'],
  'structurePane.the-column-and-everything-stored-in-it-is': [
    'The column and everything stored in it is dropped when you save.',
    '保存时该列及其中的所有数据都会被删除。',
    '儲存時此欄及其中的所有資料都會被刪除。',
  ],
  'structurePane.the-field-has-not-been-created-yet-so-it-is': [
    'The field has not been created yet, so it is simply removed from the design.',
    '该字段尚未创建，因此只是从设计中移除。',
    '此欄位尚未建立，因此只是從設計中移除。',
  ],
  'structurePane.drop': ['Drop', '删除', '刪除'],
  'structurePane.delete-field': ['Delete field', '删除字段', '刪除欄位'],
  'structurePane.field-count': ['{n} field|{n} fields', '{n} 个字段|{n} 个字段', '{n} 個欄位|{n} 個欄位'],
  'structurePane.read-only': ['read-only', '只读', '唯讀'],
  'structurePane.revert': ['Revert', '还原', '還原'],
  'structurePane.the-script-stopped-in-the-middle': [
    'The script stopped in the middle',
    '脚本中途停止',
    '指令碼中途停止',
  ],
  'structurePane.fields': ['Fields', '字段', '欄位'],
  'structurePane.collections-have-no-schema': [
    'Collections have no schema',
    '集合没有 schema',
    '集合沒有 schema',
  ],
  'structurePane.this-engine-has-no-table-designer': [
    'This engine has no table designer',
    '该引擎没有表设计器',
    '此引擎沒有資料表設計工具',
  ],
  'structurePane.the-fields-below-were-inferred-from-a-sample-of': [
    'The fields below were inferred from a sample of the documents in this collection. Any document may carry other fields, or the same field with another type, so there is nothing to design here.',
    '以下字段由该集合中文档的抽样推断得出。任何文档都可能带有其他字段，或同一字段的不同类型，因此这里无需设计。',
    '以下欄位由該集合中文件的取樣推斷得出。任何文件都可能帶有其他欄位，或同一欄位的不同型別，因此這裡無需設計。',
  ],
  'structurePane.the-fields-below-come-from-the-catalog-and-are': [
    'The fields below come from the catalog and are read-only. Doris DDL needs a data model and a distribution clause, so changes go through the DDL editor, where the engine’s own statement is edited and run.',
    '以下字段来自目录，且为只读。Doris DDL 需要数据模型和分布子句，因此变更需通过 DDL 编辑器进行，并在其中编辑和执行该引擎自己的语句。',
    '以下欄位來自目錄，且為唯讀。Doris DDL 需要資料模型與分布子句，因此變更需透過 DDL 編輯器進行，並在其中編輯與執行該引擎自己的語句。',
  ],
  'structurePane.no-documents-to-sample': ['No documents to sample', '没有可抽样的文档', '沒有可取樣的文件'],
  'structurePane.no-fields': ['No fields', '无字段', '無欄位'],
  'structurePane.field-2': ['Field', '字段', '欄位'],
  'structurePane.key': ['key', '键', '鍵'],
  'structurePane.type': ['Type', '类型', '型別'],
  'structurePane.may-be-missing': ['May be missing', '可能缺失', '可能缺少'],
  'structurePane.name': ['Name', '名称', '名稱'],
  'structurePane.columns': ['Columns', '列', '欄'],
  'structurePane.unique': ['Unique', '唯一', '唯一'],
  'structurePane.unique-2': ['unique', '唯一', '唯一'],
  'structurePane.method': ['Method', '方法', '方法'],
  'structurePane.references': ['References', '引用', '參照'],
  'structurePane.on-delete': ['On delete', '删除时', '刪除時'],
  'structurePane.on-update': ['On update', '更新时', '更新時'],
} satisfies AreaMessages
