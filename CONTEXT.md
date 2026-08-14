# HubSpot Configuration

This context describes reusable HubSpot account structure and definitions separately from operational business data and activity.

## Language

**HubSpot configuration**:
Reusable account-level structure, settings, and authored definitions across HubSpot product areas. Runtime instances and activity produced from those definitions are not HubSpot configuration.
_Avoid_: HubSpot setup, HubSpot data

**Configuration surface**:
A cohesive area of HubSpot configuration whose concepts share an identity, ownership model, permissions, and lifecycle.
_Avoid_: Feature, product area

**CRM configuration**:
Durable account-level structure that defines how CRM data is organized, including property groups, properties, pipelines, pipeline stages, and custom object schemas.
_Avoid_: CRM data, HubSpot CRM setup

**Archived CRM configuration**:
CRM configuration removed from active use but still retained by HubSpot. Archival is not equivalent to absence and is restorable only where a surface-specific API proves it.
_Avoid_: Deleted configuration, soft-deleted resource

**CRM record**:
An operational instance of a CRM object, such as a contact, deal, ticket, or custom object record. Records hold business data rather than CRM configuration.
_Avoid_: Resource, configuration object

**CRM object type**:
A category of CRM records and their shared configuration, referenced by its exact HubSpot API identifier. Standard and account-defined object types use the same concept despite different identifier forms.
_Avoid_: Object, entity

**Custom object schema**:
An account-defined CRM object type, including its labels, property roles, and relationships to other object types. Individual records of that type are CRM records, not part of the schema.
_Avoid_: Custom object, custom record

**Association label**:
A directional, account-defined classification for relationships between two CRM object types. It configures relationship meaning; individual linked records remain CRM records.
_Avoid_: Association, relationship label

**Association limit**:
A directional maximum on how many records of one CRM object type may be related through one association definition. It configures cardinality and does not represent any individual record association.
_Avoid_: Association quota, relationship count

**Association definition**:
One same-object label or an atomic forward/inverse label pair together with any directional limits attached to its generated type identities.
_Avoid_: Record association, association instance

**Property definition**:
An account-level field schema for one CRM object type, identified by an immutable internal name. Values held by CRM records are not property definitions.
_Avoid_: Property value, field value

**Property group**:
An account-level presentation grouping for property definitions of one CRM object type. It has its own internal name but is not part of a property definition's identity.
_Avoid_: Property category, field group

**CRM property schema**:
The property groups, ordinary non-sensitive property definitions, and parent-owned enumeration options for one CRM object type. The same concept applies to contacts, companies, deals, and tickets.
_Avoid_: CRM fields, property values

**Sensitive property definition**:
A property definition whose CRM record values HubSpot classifies as sensitive or highly sensitive. The definition describes that classification but contains no sensitive record values itself.
_Avoid_: Sensitive value, secret property

**Pipeline stage**:
An account-level state within one CRM pipeline, identified remotely by a HubSpot-generated ID and ordered through explicit display order. It is configuration, not a CRM record status value.
_Avoid_: Status, stage record

**Default pipeline configuration**:
The HubSpot-created singleton pipeline for one CRM object type together with its stage definitions. On HubSpot Free it is adopted and retained rather than created or removed as a whole.
_Avoid_: Pipeline CRUD, custom pipeline

**Deal pipeline configuration**:
Default pipeline configuration for deal records, whose stages express deal probability. It is distinct from ticket pipeline configuration because its permissions, identity convention, stage vocabulary, and record-reference checks differ.
_Avoid_: Sales pipeline feature

**Ticket pipeline configuration**:
Default pipeline configuration for ticket records, whose stages classify tickets as open or closed. It is distinct from deal pipeline configuration because its permissions, identity convention, stage vocabulary, and record-reference checks differ.
_Avoid_: Support pipeline feature

**Adopted pipeline stage**:
A pre-existing stage definition whose remote identity and continued existence are retained while selected mutable fields may be managed.
_Avoid_: Default stage, owned stage

**Owned pipeline stage**:
A stage definition created through declarative configuration and therefore eligible for verified removal when no CRM record still references it.
_Avoid_: Custom stage, adopted stage

**Account membership**:
An account-specific admission of a global HubSpot user, identified within the account by its Settings user ID. It is distinct from the user's CRM profile and from the global identity that may survive account removal.
_Avoid_: User account, user profile

**CRM user profile configuration**:
Account-scoped job, availability, timezone, and working-hours settings for a materialized CRM user identity joined to an account membership. It may become available only after human activation and is not the membership itself.
_Avoid_: Account membership, user preferences

**Managed file**:
A reusable File Manager asset identified by a generated HubSpot file ID, with owned bytes, metadata, access state, and folder placement. Delivery URLs and derived paths are observations rather than identity.
_Avoid_: CMS source file, file URL

**File folder**:
A generated-ID hierarchy node in HubSpot File Manager that locates managed files and child folders. Its mutable name and derived path are not identity.
_Avoid_: Filesystem directory, CMS folder

**Files configuration**:
The explicit File Manager folder hierarchy and managed files placed within it. CMS Developer File System content is a separate paid capability and is not Files configuration.
_Avoid_: CMS content, source code

**Contact segment definition**:
A reusable rule or manually maintained container for selecting contact records, identified by a generated list ID. Memberships and derived segment size are operational data rather than part of the definition.
_Avoid_: Contact list, list membership

**Manual contact segment**:
A contact segment definition with no declarative filter tree. Its membership is maintained outside configuration management.
_Avoid_: Static list

**Dynamic contact segment**:
A contact segment definition whose mutable property-filter tree is continuously evaluated by HubSpot.
_Avoid_: Active list

**Snapshot contact segment**:
A contact segment definition whose property-filter tree is fixed at creation while the definition retains a restorable identity.
_Avoid_: Static list, manual segment

**Form definition**:
A reusable authored form structure that selects CRM property definitions as fields and controls presentation before submissions occur. Submissions and their resulting CRM activity are not part of the definition.
_Avoid_: Form submission, form response

**Archived form definition**:
A form definition removed from active use but retained by HubSpot until its purge window expires. It is terminal configuration rather than a restorable active definition.
_Avoid_: Deleted form, restorable form

**Product definition**:
A reusable catalogue description of an offered product, including its SKU, name, description, price, cost, and recurring period. Line items and transactions that refer to it are operational records rather than part of the definition.
_Avoid_: Line item, product sale
