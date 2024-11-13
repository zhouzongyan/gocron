<template>
  <el-container>
    <system-sidebar></system-sidebar>
    <el-main>
      <!-- <el-pagination
        background
        layout="prev, pager, next, sizes, total"
        :total="logTotal"
        :page-size="20"
        @size-change="changePageSize"
        @current-change="changePage"
        @prev-click="changePage"
        @next-click="changePage">
      </el-pagination>
      <el-table
        :data="logs"
        border
        ref="table"
        style="width: 100%">
        <el-table-column
          prop="id"
          label="ID">
        </el-table-column>
        <el-table-column
          prop="username"
          label="用户名">
        </el-table-column>
        <el-table-column
          prop="ip"
          label="登录IP">
        </el-table-column>
        <el-table-column
          label="登录时间"
          width="">
          <template slot-scope="scope">
            {{scope.row.created | formatTime}}
          </template>
        </el-table-column>
      </el-table> -->
      <el-button type="primary" @click="startBackup()">开始备份</el-button>
      <el-link type="warning" v-if="hasBackup" @click="downloadFile()">下载备份文件</el-link>
    </el-main>
  </el-container>
</template>

<script>
import systemSidebar from './sidebar'
import systemService from '../../api/system'
import store from '../../store/index'
export default {
  name: 'backup',
  data () {
    return {
      logs: [],
      logTotal: 0,
      hasBackup: false
    }
  },
  created () {
    this.getFile()
  },
  components: {systemSidebar},
  methods: {
    startBackup () {
      const loading = this.$loading({
        lock: true,
        text: '正在备份，请勿操作其他内容',
        spinner: 'el-icon-loading',
        background: 'rgba(0, 0, 0, 0.7)'
      })
      systemService.startBackup({}, (data) => {
        loading.close()
        // console.log('备份数据返回:', data)
        if (data !== null && data.code !== 0) {
          this.$message.error(data.msg)
          return
        }
        this.$message.success('备份成功')
      })
    },
    getFile () {
      systemService.backupFile({}, (data) => {
        if (data === null) {
          this.hasBackup = true
          return
        }
        this.hasBackup = data.code === 0
      })
    },
    downloadFile () {
      window.open('/api/system/backup/download?Auth-Token=' + store.getters.user.token)
    },
    search () {
      systemService.loginLogList(this.searchParams, (data) => {
        this.logs = data.data
        this.logTotal = data.total
      })
    }
  }
}
</script>
