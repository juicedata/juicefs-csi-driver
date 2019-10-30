variable "project" {}
variable "region" {}

provider "google" {
  version = "~> 2.18"
  project = var.project
  region  = var.region
}

resource "google_service_account" "this" {
  account_id   = "juicefs-csi-driver-validation"
  display_name = "JuiceFS CSI driver validation"
}

resource "google_storage_bucket" "this" {
  name          = "juicefs-csi-driver-valiation"
  location      = var.region
  storage_class = "REGIONAL"
}

resource "google_storage_bucket_iam_member" "member" {
  bucket = google_storage_bucket.this.name
  role   = "roles/storage.admin"
  member = "serviceAccount:${google_service_account.this.email}"
}
