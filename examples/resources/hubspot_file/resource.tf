resource "hubspot_file" "logo" {
  name          = "northstar-logo.svg"
  folder_id     = hubspot_file_folder.brand.id
  source_path   = "${path.module}/northstar-logo.svg"
  source_sha256 = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
  access        = "PUBLIC_NOT_INDEXABLE"
}
