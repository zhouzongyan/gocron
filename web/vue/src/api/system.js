import httpClient from '../utils/httpClient'

export default {
  loginLogList (query, callback) {
    httpClient.get('/system/login-log', query, callback)
  },
  startBackup (query, callback) {
    httpClient.get('/system/backup/start', query, callback)
  },
  backupFile (query, callback) {
    httpClient.get('/system/backup/file', query, callback)
  }
}
