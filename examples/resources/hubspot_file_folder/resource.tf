resource "hubspot_file_folder" "assets" {
  name = "Northstar assets"
}

resource "hubspot_file_folder" "brand" {
  name             = "Brand"
  parent_folder_id = hubspot_file_folder.assets.id
}
