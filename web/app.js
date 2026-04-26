const App = {
  state: {
    token: localStorage.getItem('cdl_token') || '',
    refreshToken: localStorage.getItem('cdl_refresh_token') || '',
    currentUser: null,
    loginError: '',
    dashboard: null,
    projects: { list: [], total: 0, page: 1, page_size: 20 },
    issueSummary: { list: [], total: 0, page: 1, page_size: 100 },
    projectFilter: {
      keyword: '',
      customer_name: '',
      project_status: '',
      current_version: '',
      page: 1,
      page_size: 20,
    },
    issueFilter: {
      keyword: '',
      issue_version: '',
      page: 1,
      page_size: 100,
    },
    selectedProject: null,
    projectOverview: null,
    projectTab: 'overview',
    changeTab: 'upgrades',
    serviceTab: 'services',
    records: {
      upgrades: [],
      configs: [],
      sqls: [],
      integrations: [],
      assets: [],
      serviceRecords: [],
      attachments: [],
      auditLogs: [],
    },
    users: [],
    dashboardFilter: {
      month: currentMonthValue(),
    },
    currentView: 'projects',
    modal: null,
    toast: '',
    projectLoading: false,
    tabLoading: false,
    bootstrapping: !!localStorage.getItem('cdl_token'),
  },

  apiBase: '/api/v1',

  init() {
    this.syncRoute();
    window.addEventListener('popstate', () => this.handlePopState());
    this.render();
    if (this.state.token) {
      this.bootstrap();
    }
  },

  async bootstrap() {
    this.state.bootstrapping = true;
    this.render();
    try {
      await this.fetchMe();
      const tasks = [this.loadDashboard(), this.loadProjects()];
      if (this.state.currentView === 'issues') {
        tasks.push(this.loadIssueSummary());
      }
      if (this.state.currentView === 'users') {
        tasks.push(this.loadUsers());
      }
      await Promise.all(tasks);
      this.syncRoute();
      this.render();
    } catch (error) {
      this.logout();
    } finally {
      this.state.bootstrapping = false;
      this.render();
    }
  },

  async api(path, options = {}) {
    const headers = options.headers || {};
    if (!(options.body instanceof FormData)) {
      headers['Content-Type'] = 'application/json';
    }
    if (this.state.token) {
      headers['Authorization'] = `Bearer ${this.state.token}`;
    }
    const response = await fetch(`${this.apiBase}${path}`, {
      ...options,
      headers,
    });
    if (response.status === 401 && this.state.refreshToken && path !== '/auth/refresh') {
      const refreshed = await this.tryRefreshToken();
      if (refreshed) {
        return this.api(path, options);
      }
    }
    const data = await response.json().catch(() => ({}));
    if (!response.ok || data.code !== 0) {
      throw new Error(data.message || '请求失败');
    }
    return data.data;
  },

  async tryRefreshToken() {
    try {
      const response = await fetch(`${this.apiBase}/auth/refresh`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ refresh_token: this.state.refreshToken }),
      });
      const data = await response.json();
      if (response.ok && data.code === 0) {
        this.state.token = data.data.access_token;
        localStorage.setItem('cdl_token', this.state.token);
        return true;
      }
    } catch (_) {
      // ignore
    }
    return false;
  },

  async login(event) {
    event.preventDefault();
    const form = event.target;
    this.state.loginError = '';
    this.render();
    try {
      const data = await this.api('/auth/login', {
        method: 'POST',
        body: JSON.stringify({
          username: form.username.value.trim(),
          password: form.password.value,
        }),
      });
      this.state.token = data.access_token;
      this.state.refreshToken = data.refresh_token;
      this.state.currentUser = data.user;
      this.state.loginError = '';
      localStorage.setItem('cdl_token', this.state.token);
      localStorage.setItem('cdl_refresh_token', this.state.refreshToken);
      await Promise.all([this.loadDashboard(), this.loadProjects()]);
      this.navigateTo('/');
      this.showToast('登录成功');
      this.render();
    } catch (error) {
      this.state.loginError = this.normalizeLoginError(error.message);
      this.render();
    }
  },

  logout() {
    this.state.token = '';
    this.state.refreshToken = '';
    this.state.currentUser = null;
    this.state.loginError = '';
    this.state.selectedProject = null;
    this.state.projectOverview = null;
    this.state.currentView = 'projects';
    this.state.serviceTab = 'services';
    this.state.bootstrapping = false;
    localStorage.removeItem('cdl_token');
    localStorage.removeItem('cdl_refresh_token');
    this.navigateTo('/login', true);
    this.render();
  },

  async fetchMe() {
    this.state.currentUser = await this.api('/auth/me');
  },

  async loadDashboard() {
    const query = new URLSearchParams(this.state.dashboardFilter).toString();
    this.state.dashboard = await this.api(`/dashboard/overview?${query}`);
  },

  async loadProjects() {
    const query = new URLSearchParams(this.state.projectFilter).toString();
    this.state.projects = await this.api(`/projects?${query}`);
    if (this.state.selectedProject) {
      const fresh = this.state.projects.list.find(item => item.id === this.state.selectedProject.id);
      if (fresh) {
        this.state.selectedProject = fresh;
      }
    }
  },

  async loadIssueSummary() {
    const query = new URLSearchParams(this.state.issueFilter).toString();
    this.state.issueSummary = await this.api(`/issues?${query}`);
  },

  async openProjectDetail(projectId) {
    const draft = (this.state.projects.list || []).find(item => item.id === projectId);
    this.state.selectedProject = draft ? { ...draft } : { id: projectId };
    this.state.projectTab = 'overview';
    this.state.changeTab = 'upgrades';
    this.state.serviceTab = 'services';
    this.state.projectOverview = null;
    this.state.projectLoading = true;
    this.state.tabLoading = true;
    this.render();
    try {
      this.state.selectedProject = await this.api(`/projects/${projectId}`);
      await Promise.all([
        this.loadProjectOverview(),
        this.loadCurrentTabData(),
      ]);
    } catch (error) {
      this.state.selectedProject = null;
      this.state.projectOverview = null;
      this.showToast(error.message || '加载项目失败');
    } finally {
      this.state.projectLoading = false;
      this.state.tabLoading = false;
      this.render();
    }
  },

  async loadProjectOverview() {
    if (!this.state.selectedProject) return;
    this.state.projectOverview = await this.api(`/projects/${this.state.selectedProject.id}/overview`);
  },

  async loadCurrentTabData() {
    if (!this.state.selectedProject) return;
    const projectId = this.state.selectedProject.id;
    if (this.state.projectTab === 'overview') {
      await this.loadProjectOverview();
      return;
    }
    if (this.state.projectTab === 'changes') {
      if (this.state.changeTab === 'upgrades') {
        const res = await this.api(`/projects/${projectId}/upgrades?page=1&page_size=100`);
        this.state.records.upgrades = res.list;
      }
      if (this.state.changeTab === 'configs') {
        const res = await this.api(`/projects/${projectId}/config-changes?page=1&page_size=100`);
        this.state.records.configs = res.list;
      }
      if (this.state.changeTab === 'sqls') {
        const res = await this.api(`/projects/${projectId}/sql-changes?page=1&page_size=100`);
        this.state.records.sqls = res.list;
      }
      if (this.state.changeTab === 'assets') {
        const res = await this.api(`/projects/${projectId}/assets?page=1&page_size=100`);
        this.state.records.assets = res.list;
      }
    }
    if (this.state.projectTab === 'integrations') {
      const res = await this.api(`/projects/${projectId}/integrations?page=1&page_size=100`);
      this.state.records.integrations = res.list;
    }
    if (this.state.projectTab === 'services') {
      const res = await this.api(`/projects/${projectId}/service-records?page=1&page_size=100`);
      this.state.records.serviceRecords = res.list;
    }
    if (this.state.projectTab === 'attachments') {
      const res = await this.api(`/projects/${projectId}/attachments?page=1&page_size=100`);
      this.state.records.attachments = res.list;
    }
    if (this.state.projectTab === 'audit') {
      const res = await this.api(`/projects/${projectId}/audit-logs?page=1&page_size=100`);
      this.state.records.auditLogs = res.list;
    }
  },

  async loadUsers() {
    this.state.users = await this.api('/users');
  },

  async switchView(view) {
    this.state.currentView = view;
    if (view === 'dashboard') {
      this.state.selectedProject = null;
      this.state.projectOverview = null;
      await this.loadDashboard();
    }
    if (view === 'projects') {
      this.state.selectedProject = null;
      this.state.projectOverview = null;
      this.state.projectTab = 'overview';
      this.state.changeTab = 'upgrades';
      this.state.serviceTab = 'services';
      await this.loadProjects();
    }
    if (view === 'issues') {
      this.state.selectedProject = null;
      this.state.projectOverview = null;
      this.state.projectTab = 'overview';
      this.state.changeTab = 'upgrades';
      this.state.serviceTab = 'issues';
      await this.loadIssueSummary();
    }
    if (view === 'users') {
      await this.loadUsers();
    }
    this.navigateTo(view === 'projects' ? '/' : `/${view}`);
    this.render();
  },

  async switchProjectTab(tab) {
    this.state.projectTab = tab;
    this.state.tabLoading = true;
    this.render();
    try {
      await this.loadCurrentTabData();
    } finally {
      this.state.tabLoading = false;
      this.render();
    }
  },

  async switchChangeTab(tab) {
    this.state.changeTab = tab;
    this.state.tabLoading = true;
    this.render();
    try {
      await this.loadCurrentTabData();
    } finally {
      this.state.tabLoading = false;
      this.render();
    }
  },

  switchServiceTab(tab) {
    this.state.serviceTab = tab;
    this.render();
  },

  updateDashboardFilter(key, value) {
    this.state.dashboardFilter[key] = value;
  },

  updateIssueFilter(key, value) {
    this.state.issueFilter[key] = value;
  },

  async submitDashboardFilter(event) {
    event.preventDefault();
    await this.loadDashboard();
    this.render();
  },

  async submitIssueFilter(event) {
    event.preventDefault();
    this.state.issueFilter.page = 1;
    await this.loadIssueSummary();
    this.render();
  },

  async resetIssueFilter() {
    this.state.issueFilter = {
      keyword: '',
      issue_version: '',
      page: 1,
      page_size: 100,
    };
    await this.loadIssueSummary();
    this.render();
  },

  normalizeLoginError(message) {
    const text = String(message || '').trim();
    const lower = text.toLowerCase();
    if (!text) return '登录失败，请稍后重试';
    if (
      lower.includes('record not found')
      || text.includes('记录不存在')
      || text.includes('用户不存在')
      || text.includes('密码错误')
      || text.includes('账号或密码错误')
    ) {
      return '账号或密码错误';
    }
    if (text.includes('停用')) {
      return '账号已停用，请联系管理员';
    }
    return text;
  },

  syncRoute() {
    const path = this.currentPath();
    if (!this.state.token) {
      if (path !== '/login') {
        this.navigateTo('/login', true);
      }
      return;
    }
    if (path === '/dashboard') {
      this.state.currentView = 'dashboard';
    } else if (path === '/issues') {
      this.state.currentView = 'issues';
    } else if (path === '/users') {
      this.state.currentView = 'users';
    } else {
      this.state.currentView = 'projects';
    }
    if (path === '/login') {
      this.state.currentView = 'projects';
      this.navigateTo('/', true);
    }
  },

  handlePopState() {
    const path = this.currentPath();
    if (path === '/login') {
      if (this.state.token) {
        this.navigateTo('/', true);
      }
      this.render();
      return;
    }
    if (path === '/dashboard') {
      this.switchView('dashboard');
      return;
    }
    if (path === '/issues') {
      this.switchView('issues');
      return;
    }
    if (path === '/users') {
      this.switchView('users');
      return;
    }
    if (this.state.currentView !== 'projects') {
      this.switchView('projects');
      return;
    }
    this.render();
  },

  currentPath() {
    const path = window.location.pathname || '/';
    return path.length > 1 && path.endsWith('/') ? path.slice(0, -1) : path;
  },

  navigateTo(path, replace = false) {
    const current = this.currentPath();
    if (current === path) return;
    const method = replace ? 'replaceState' : 'pushState';
    window.history[method]({}, '', path);
  },

  updateProjectFilter(key, value) {
    this.state.projectFilter[key] = value;
  },

  async submitProjectFilter(event) {
    event.preventDefault();
    this.state.projectFilter.page = 1;
    await this.loadProjects();
    this.render();
  },

  async resetProjectFilter() {
    this.state.projectFilter = {
      keyword: '',
      customer_name: '',
      project_status: '',
      current_version: '',
      page: 1,
      page_size: 20,
    };
    await this.loadProjects();
    this.render();
  },

  openProjectForm(project) {
    const editing = !!project;
    const fields = [
      { name: 'project_name', label: '项目名称', required: true, value: project?.project_name || '' },
      { name: 'customer_name', label: '客户名称', required: true, value: project?.customer_name || '' },
      { name: 'project_status', label: '项目状态', type: 'select', required: true, value: project?.project_status || 'implementing', options: this.projectStatusOptions() },
      { name: 'implementation_date', label: '实施日期', type: 'date', required: true, value: project?.implementation_date || todayDate() },
      { name: 'online_date', label: '上线日期', type: 'date', value: project?.online_date || '' },
      { name: 'acceptance_date', label: '验收日期', type: 'date', value: project?.acceptance_date || '' },
      { name: 'current_version', label: '当前版本', required: true, value: project?.current_version || '' },
      { name: 'deploy_mode', label: '部署环境', type: 'select', value: project?.deploy_mode || 'standalone', options: this.deployModeOptions() },
      { name: 'customer_contact', label: '客户联系人', value: project?.customer_contact || '' },
      ...(editing ? [{ name: 'environment_summary', label: '环境说明', type: 'textarea', value: project?.environment_summary || '' }] : []),
      { name: 'remark_text', label: '备注', type: 'textarea', value: project?.remark_text || '' },
    ];
    this.openFormModal({
      title: editing ? '编辑项目' : '新建项目',
      fields,
      skipUnchangedSubmit: editing,
      attachmentContext: editing ? { projectId: project.id, refType: 'project', refId: project.id } : null,
      onSubmit: async (values, modal) => {
        let savedProject;
        if (editing) {
          savedProject = await this.api(`/projects/${project.id}`, { method: 'PUT', body: JSON.stringify({ ...project, ...values, id: project.id }) });
          await this.uploadRemarkScreenshots(project.id, 'project', project.id, modal?.pastedImages || []);
          this.showToast('项目已更新');
        } else {
          savedProject = await this.api('/projects', { method: 'POST', body: JSON.stringify(values) });
          await this.uploadRemarkScreenshots(savedProject.id, 'project', savedProject.id, modal?.pastedImages || []);
          this.showToast('项目已创建');
        }
        this.closeModal(true);
        await Promise.all([this.loadProjects(), this.loadDashboard()]);
        if (editing && this.state.selectedProject?.id === project.id) {
          await this.openProjectDetail(project.id);
          return;
        }
        this.render();
      },
    });
  },

  openUserForm(user) {
    const editing = !!user;
    this.openFormModal({
      title: editing ? '编辑用户' : '新增用户',
      fields: [
        { name: 'username', label: '登录账号', required: true, disabled: editing, submit: !editing, value: user?.username || '' },
        { name: 'real_name', label: '真实姓名', required: true, value: user?.real_name || '' },
        { name: 'password', label: editing ? '新密码（留空不修改）' : '初始密码', type: 'password', required: !editing, value: '' },
        { name: 'status', label: '启用状态', type: 'select', required: true, value: user?.status ? 'true' : 'false', options: [{ value: 'true', label: '启用' }, { value: 'false', label: '停用' }] },
      ],
      skipUnchangedSubmit: editing,
      prepareSubmitValues: values => ({
        real_name: (values.real_name || '').trim(),
        status: values.status === 'true',
        password: values.password || '',
      }),
      onSubmit: async values => {
        const payload = {
          real_name: (values.real_name || '').trim(),
          status: values.status === 'true',
        };
        const password = values.password || '';
        if (editing) {
          await this.api(`/users/${user.id}`, { method: 'PUT', body: JSON.stringify(payload) });
          if (password) {
            await this.api(`/users/${user.id}/reset-password`, { method: 'POST', body: JSON.stringify({ new_password: password }) });
          }
          this.showToast('用户已更新');
        } else {
          await this.api('/users', {
            method: 'POST',
            body: JSON.stringify({
              username: (values.username || '').trim(),
              real_name: payload.real_name,
              password,
              status: payload.status,
            }),
          });
          this.showToast('用户已创建');
        }
        this.closeModal(true);
        await this.loadUsers();
        this.render();
      },
    });
  },

  openRecordForm(type, record) {
    const projectId = this.state.selectedProject?.id;
    if (!projectId) return;
    const editing = !!record;
    const config = this.recordFormConfig(type, record);
    const refType = this.recordAttachmentType(type);
    this.openFormModal({
      title: `${editing ? '编辑' : '新增'}${config.title}`,
      fields: config.fields,
      skipUnchangedSubmit: editing,
      prepareSubmitValues: values => this.normalizeRecordValues(type, values),
      attachmentContext: editing ? { projectId, refType, refId: record.id } : { projectId, refType },
      onSubmit: async (values, modal) => {
        values = this.normalizeRecordValues(type, values);
        let savedRecord;
        if (editing) {
          savedRecord = await this.api(`${config.singleEndpoint}/${record.id}`, {
            method: 'PUT',
            body: JSON.stringify({ ...record, ...values }),
          });
          await this.uploadRemarkScreenshots(projectId, refType, record.id, modal?.pastedImages || []);
          this.showToast(`${config.title}已更新`);
        } else {
          savedRecord = await this.api(`/projects/${projectId}${config.collectionEndpoint}`, {
            method: 'POST',
            body: JSON.stringify(values),
          });
          await this.uploadRemarkScreenshots(projectId, refType, savedRecord.id, modal?.pastedImages || []);
          this.showToast(`${config.title}已创建`);
        }
        this.closeModal(true);
        await Promise.all([this.loadCurrentTabData(), this.loadProjectOverview(), this.loadProjects()]);
        this.render();
      },
    });
  },

  async deleteRecord(type, id) {
    const config = this.recordFormConfig(type);
    if (!confirm(`确认删除这条${config.title}吗？`)) return;
    try {
      await this.api(`${config.singleEndpoint}/${id}`, { method: 'DELETE' });
      this.showToast(`${config.title}已删除`);
      await Promise.all([this.loadCurrentTabData(), this.loadProjectOverview(), this.loadProjects()]);
      this.render();
    } catch (error) {
      this.showToast(error.message);
    }
  },

  openAttachmentForm() {
    const projectId = this.state.selectedProject?.id;
    if (!projectId) return;
    this.cleanupModalResources();
    this.state.modal = {
      type: 'attachment',
      title: '上传附件',
    };
    this.render();
  },

  async submitAttachment(event) {
    event.preventDefault();
    const projectId = this.state.selectedProject?.id;
    if (!projectId) return;
    const form = event.target;
    const file = form.file.files[0];
    if (!file) {
      this.showToast('请选择附件');
      return;
    }
    const data = new FormData();
    data.append('file', file);
    data.append('title', form.title.value.trim());
    data.append('doc_category', form.doc_category.value);
    data.append('ref_type', form.ref_type.value || 'project');
    data.append('ref_id', form.ref_id.value || '0');
    data.append('tags', form.tags.value || '');
    data.append('description', form.description.value || '');
    try {
      await this.api(`/projects/${projectId}/attachments`, {
        method: 'POST',
        body: data,
      });
      this.showToast('附件上传成功');
      this.closeModal(true);
      await Promise.all([this.loadCurrentTabData(), this.loadProjectOverview(), this.loadProjects()]);
      this.render();
    } catch (error) {
      this.showToast(error.message);
    }
  },

  async previewAttachment(id) {
    try {
      const res = await fetch(`${this.apiBase}/attachments/${id}/preview`, {
        headers: { Authorization: `Bearer ${this.state.token}` },
      });
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      window.open(url, '_blank');
      setTimeout(() => URL.revokeObjectURL(url), 10000);
    } catch (error) {
      this.showToast('附件预览失败');
    }
  },

  async downloadAttachment(id) {
    try {
      const attachment = await this.api(`/attachments/${id}`);
      const res = await fetch(`${this.apiBase}/attachments/${id}/download`, {
        headers: { Authorization: `Bearer ${this.state.token}` },
      });
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = attachment.original_name || attachment.file_name;
      document.body.appendChild(a);
      a.click();
      a.remove();
      setTimeout(() => URL.revokeObjectURL(url), 10000);
    } catch (error) {
      this.showToast('附件下载失败');
    }
  },

  async deleteAttachment(id) {
    if (!confirm('确认删除这份附件吗？')) return;
    try {
      await this.api(`/attachments/${id}`, { method: 'DELETE' });
      this.showToast('附件已删除');
      await Promise.all([this.loadCurrentTabData(), this.loadProjectOverview(), this.loadProjects()]);
      this.render();
    } catch (error) {
      this.showToast(error.message);
    }
  },

  recordAttachmentType(type) {
    const mapping = {
      upgrades: 'upgrade',
      configs: 'config',
      sqls: 'sql',
      integrations: 'integration',
      assets: 'asset',
      services: 'service',
      issues: 'service',
    };
    return mapping[type] || 'project';
  },

  cleanupModalResources(modal = this.state.modal) {
    if (!modal || modal.type !== 'form' || !Array.isArray(modal.pastedImages)) return;
    modal.pastedImages.forEach(item => {
      if (item?.previewUrl) {
        URL.revokeObjectURL(item.previewUrl);
      }
    });
  },

  async loadModalScreenshots(modal) {
    const context = modal?.attachmentContext;
    if (!context?.projectId || !context?.refId) return;
    modal.loadingScreenshots = true;
    this.render();
    try {
      const result = await this.api(`/projects/${context.projectId}/attachments?ref_type=${encodeURIComponent(context.refType)}&ref_id=${context.refId}&doc_category=screenshot&page=1&page_size=50`);
      if (this.state.modal !== modal) return;
      modal.existingScreenshots = result.list || [];
    } catch (_) {
      if (this.state.modal !== modal) return;
      modal.existingScreenshots = [];
    } finally {
      if (this.state.modal === modal) {
        modal.loadingScreenshots = false;
        this.render();
      }
    }
  },

  handleRemarkPaste(event) {
    const modal = this.state.modal;
    if (!modal || modal.type !== 'form') return;
    const items = Array.from(event.clipboardData?.items || []).filter(item => item.type.startsWith('image/'));
    if (!items.length) return;
    event.preventDefault();
    const now = Date.now();
    const images = items.map((item, index) => {
      const file = item.getAsFile();
      if (!file) return null;
      const ext = (file.type.split('/')[1] || 'png').replace(/[^a-z0-9]/gi, '').toLowerCase() || 'png';
      return {
        id: `${now}-${index}-${Math.random().toString(16).slice(2)}`,
        file,
        previewUrl: URL.createObjectURL(file),
        fileName: `remark-screenshot-${now}-${index + 1}.${ext}`,
      };
    }).filter(Boolean);
    if (!images.length) return;
    modal.pastedImages = [...(modal.pastedImages || []), ...images];
    this.showToast(`已粘贴 ${images.length} 张截图，保存后会自动上传`);
    this.render();
  },

  removePastedImage(id) {
    const modal = this.state.modal;
    if (!modal || modal.type !== 'form') return;
    const target = (modal.pastedImages || []).find(item => item.id === id);
    if (target?.previewUrl) {
      URL.revokeObjectURL(target.previewUrl);
    }
    modal.pastedImages = (modal.pastedImages || []).filter(item => item.id !== id);
    this.render();
  },

  async uploadRemarkScreenshots(projectId, refType, refId, images) {
    if (!projectId || !refId || !images?.length) return;
    try {
      for (const [index, item] of images.entries()) {
        const data = new FormData();
        data.append('file', item.file, item.fileName || `remark-screenshot-${index + 1}.png`);
        data.append('title', `备注截图 ${index + 1}`);
        data.append('doc_category', 'screenshot');
        data.append('ref_type', refType || 'project');
        data.append('ref_id', String(refId));
        data.append('description', '由备注区域粘贴上传');
        await this.api(`/projects/${projectId}/attachments`, {
          method: 'POST',
          body: data,
        });
      }
    } catch (error) {
      this.showToast(`记录已保存，但截图上传失败：${error.message || '请稍后重试'}`);
    }
  },

  async openAuditView() {
    this.state.projectTab = 'audit';
    await this.loadCurrentTabData();
    this.render();
  },

  openFormModal({ title, fields, onSubmit, attachmentContext, skipUnchangedSubmit = false, prepareSubmitValues = null }) {
    this.cleanupModalResources();
    const initialFormValues = this.buildModalFormValues(fields);
    const modal = {
      type: 'form',
      title,
      fields,
      onSubmit,
      skipUnchangedSubmit,
      prepareSubmitValues,
      attachmentContext: attachmentContext || null,
      initialFormValues,
      formValues: { ...initialFormValues },
      initialSubmitValues: this.buildModalSubmitValues(fields, prepareSubmitValues),
      pastedImages: [],
      existingScreenshots: [],
      loadingScreenshots: false,
    };
    this.state.modal = modal;
    this.render();
    if (modal.attachmentContext?.projectId && modal.attachmentContext?.refId) {
      this.loadModalScreenshots(modal);
    }
  },

  closeModal(force = false) {
    const modal = this.state.modal;
    if (!force && this.shouldConfirmModalClose(modal) && !confirm('当前内容尚未保存，确认关闭吗？')) {
      return;
    }
    this.cleanupModalResources();
    this.state.modal = null;
    this.render();
  },

  async submitModalForm(event) {
    event.preventDefault();
    const modal = this.state.modal;
    if (!modal || modal.type !== 'form') return;
    const values = this.collectFormValues(event.target, modal.fields);
    const comparableValues = this.prepareModalSubmitValues(modal, values);
    if (modal.skipUnchangedSubmit && !this.hasModalSubmitChanges(modal, comparableValues)) {
      this.closeModal(true);
      this.showToast('未检测到变更');
      return;
    }
    try {
      await modal.onSubmit(values, modal);
    } catch (error) {
      this.showToast(error.message || '提交失败');
    }
  },

  buildModalFormValues(fields) {
    const values = {};
    (fields || []).forEach(field => {
      values[field.name] = this.formatFieldValue(field.type, field.value ?? '');
    });
    return values;
  },

  buildModalSubmitValues(fields, prepareSubmitValues = null) {
    const values = {};
    (fields || []).forEach(field => {
      if (field.submit === false || field.disabled) {
        return;
      }
      values[field.name] = this.normalizeFieldSubmitValue(field, field.value ?? '');
    });
    return prepareSubmitValues ? prepareSubmitValues(values) : values;
  },

  updateModalDraft(name, value) {
    const modal = this.state.modal;
    if (!modal || modal.type !== 'form') return;
    if (!modal.formValues) {
      modal.formValues = {};
    }
    modal.formValues[name] = value;
  },

  shouldConfirmModalClose(modal = this.state.modal) {
    if (!modal || modal.type !== 'form') return false;
    const initial = JSON.stringify(modal.initialFormValues || {});
    const current = JSON.stringify(modal.formValues || {});
    if (initial !== current) {
      return true;
    }
    return Array.isArray(modal.pastedImages) && modal.pastedImages.length > 0;
  },

  hasModalSubmitChanges(modal, values) {
    if (!modal || modal.type !== 'form') return true;
    if (Array.isArray(modal.pastedImages) && modal.pastedImages.length > 0) {
      return true;
    }
    const initial = JSON.stringify(modal.initialSubmitValues || {});
    const current = JSON.stringify(values || {});
    return initial !== current;
  },

  prepareModalSubmitValues(modal, values) {
    if (!modal || modal.type !== 'form') return values;
    if (typeof modal.prepareSubmitValues === 'function') {
      return modal.prepareSubmitValues(values);
    }
    return values;
  },

  collectFormValues(form, fields) {
    const values = {};
    fields.forEach(field => {
      if (field.submit === false || field.disabled) {
        return;
      }
      values[field.name] = this.normalizeFieldSubmitValue(field, form[field.name]?.value ?? '');
    });
    return values;
  },

  normalizeFieldSubmitValue(field, value) {
    if (field.type === 'number' && value !== '') {
      return Number(value);
    }
    if (field.type === 'datetime-local' && value) {
      return new Date(value).toISOString();
    }
    return value;
  },

  normalizeRecordValues(type, values) {
    const config = this.recordFormConfig(type);
    const allowedNames = new Set((config?.fields || []).map(field => field.name));
    const intFieldsByType = {
      integrations: new Set(['internal_owner_user_id']),
    };
    const normalized = {};
    Object.entries(values || {}).forEach(([key, value]) => {
      if (!allowedNames.has(key)) {
        return;
      }
      if (intFieldsByType[type]?.has(key)) {
        normalized[key] = value === '' || value === undefined || value === null ? 0 : Number(value);
        return;
      }
      normalized[key] = value;
    });
    return normalized;
  },

  async backToProjects() {
    this.state.selectedProject = null;
    this.state.projectOverview = null;
    this.state.projectTab = 'overview';
    this.state.serviceTab = 'services';
    this.navigateTo('/', true);
    this.render();
  },

  async openIssueSummaryProject(projectId) {
    await this.switchView('projects');
    await this.openProjectDetail(projectId);
    await this.switchProjectTab('services');
    this.switchServiceTab('issues');
  },

  showToast(message) {
    this.state.toast = message;
    this.render();
    clearTimeout(this._toastTimer);
    this._toastTimer = setTimeout(() => {
      this.state.toast = '';
      this.render();
    }, 2600);
  },

  render() {
    const root = document.getElementById('app');
    if (this.state.token && this.state.bootstrapping && !this.state.currentUser) {
      root.innerHTML = this.renderBootstrapping();
      return;
    }
    if (!this.state.token || !this.state.currentUser) {
      root.innerHTML = this.renderLogin();
      return;
    }
    root.innerHTML = `
      <div class="page-shell">
        ${this.renderHeader()}
        <div class="container">
          ${this.renderCurrentView()}
        </div>
        ${this.renderModal()}
        ${this.state.toast ? `<div class="toast">${escapeHtml(this.state.toast)}</div>` : ''}
      </div>
    `;
  },

  renderCurrentView() {
    if (this.state.currentView === 'dashboard') {
      return this.renderDashboardView();
    }
    if (this.state.currentView === 'issues') {
      return this.renderIssueSummaryView();
    }
    if (this.state.currentView === 'users') {
      return this.renderUsersView();
    }
    return this.renderProjectsView();
  },

  renderBootstrapping() {
    return `
      <div class="page-shell page-loading-shell">
        <div class="loading-card card">
          <div class="loading-title">正在校验登录状态</div>
          <div class="muted">请稍候，系统正在加载数据。</div>
        </div>
      </div>
    `;
  },

  renderLogin() {
    return `
      <div class="login-shell">
        <section class="login-hero">
          <h1>现场交付档案中心</h1>
          <p>面向交付、实施、运维和研发支持团队的客户现场档案平台，统一沉淀项目版本、升级记录、配置与 SQL 变更、脚本资产和交付资料。</p>
          <p>请使用公司分配的内部账号登录。首次部署后请尽快修改初始化管理员密码。</p>
        </section>
        <section class="login-card-wrap">
          <form class="card login-card" onsubmit="App.login(event)">
            <h2 class="section-title">登录系统</h2>
            <div class="field">
              <label class="required">账号</label>
              <input name="username" autocomplete="username" placeholder="请输入账号" />
            </div>
            <div class="field">
              <label class="required">密码</label>
              <input type="password" name="password" autocomplete="current-password" placeholder="请输入密码" />
            </div>
            ${this.state.loginError ? `<div class="form-error">${escapeHtml(this.state.loginError)}</div>` : ''}
            <button class="btn btn-primary btn-block" type="submit">登录</button>
          </form>
        </section>
      </div>
    `;
  },

  renderHeader() {
    return `
      <header class="header">
        <div class="brand">
          <div class="brand-badge">档</div>
          <div>
            <div><strong>现场交付档案中心</strong></div>
            <div class="muted">客户现场版本、变更与资料统一归档。</div>
          </div>
        </div>
        <div class="nav-actions">
          ${this.renderViewButton('dashboard', '看板')}
          ${this.renderViewButton('projects', '项目档案')}
          ${this.renderViewButton('issues', '问题汇总')}
          ${this.renderViewButton('users', '用户管理')}
          <span class="muted">${escapeHtml(this.state.currentUser.real_name || this.state.currentUser.username)}</span>
          <button class="btn btn-primary" type="button" onclick="App.logout()">退出</button>
        </div>
      </header>
    `;
  },

  renderViewButton(view, label) {
    const className = this.state.currentView === view ? 'btn btn-primary' : 'btn btn-secondary';
    return `<button class="${className}" type="button" onclick="App.switchView('${view}')">${label}</button>`;
  },

  renderPageIntro(title, description, actions = '') {
    return `
      <div class="page-intro card">
        <div class="page-intro-head">
          <div>
            <h2 class="section-title section-title-tight">${escapeHtml(title)}</h2>
            <div class="section-subtitle">${escapeHtml(description)}</div>
          </div>
          ${actions ? `<div class="page-intro-actions">${actions}</div>` : ''}
        </div>
      </div>
    `;
  },

  renderProjectsView() {
    if (this.state.selectedProject) {
      return this.renderProjectDetail();
    }
    return `
      ${this.renderPageIntro(
        '项目档案',
        '按项目名称快速检索，进入详情后集中查看升级、配置、SQL、外部对接、脚本资产、附件和问题记录。',
        this.canEdit() ? `<button class="btn btn-primary" type="button" onclick="App.openProjectForm()">新建项目</button>` : ''
      )}
      <form class="filters" onsubmit="App.submitProjectFilter(event)">
        <div class="field">
          <label>关键字</label>
          <input value="${escapeAttr(this.state.projectFilter.keyword)}" oninput="App.updateProjectFilter('keyword', this.value)" placeholder="项目名称 / 客户名称" />
        </div>
        <div class="field">
          <label>客户名称</label>
          <input value="${escapeAttr(this.state.projectFilter.customer_name)}" oninput="App.updateProjectFilter('customer_name', this.value)" />
        </div>
        <div class="field">
          <label>项目状态</label>
          <select onchange="App.updateProjectFilter('project_status', this.value)">
            ${this.renderOptions([{ value: '', label: '全部' }, ...this.projectStatusOptions()], this.state.projectFilter.project_status)}
          </select>
        </div>
        <div class="field">
          <label>当前版本</label>
          <input value="${escapeAttr(this.state.projectFilter.current_version)}" oninput="App.updateProjectFilter('current_version', this.value)" />
        </div>
        <div class="field field-actions">
          <button class="btn btn-primary" type="submit">查询</button>
          <button class="btn btn-secondary" type="button" onclick="App.resetProjectFilter()">重置</button>
        </div>
      </form>
      ${this.renderProjectTable()}
    `;
  },

  renderDashboardView() {
    const d = this.state.dashboard || { selected_month: this.state.dashboardFilter.month, version_stats: [], issue_version_stats: [] };
    return `
      ${this.renderPageIntro('交付看板', '按月份查看问题数、服务记录数、升级次数以及当前现场版本与问题版本分布。')}
      <form class="filters" onsubmit="App.submitDashboardFilter(event)">
        <div class="field">
          <label>统计月份</label>
          <input type="month" value="${escapeAttr(this.state.dashboardFilter.month)}" oninput="App.updateDashboardFilter('month', this.value)" />
        </div>
        <div class="field field-actions">
          <button class="btn btn-primary" type="submit">刷新看板</button>
        </div>
      </form>
      <div class="stats-grid">
        <div class="stat-card"><span class="muted">项目总数</span><strong>${d.project_total || 0}</strong></div>
        <div class="stat-card"><span class="muted">维护中项目</span><strong>${d.maintenance_project_num || 0}</strong></div>
        <div class="stat-card"><span class="muted">${escapeHtml(d.selected_month || this.state.dashboardFilter.month)} 升级次数</span><strong>${d.monthly_upgrade_num || 0}</strong></div>
        <div class="stat-card"><span class="muted">${escapeHtml(d.selected_month || this.state.dashboardFilter.month)} 服务记录</span><strong>${d.monthly_service_num || 0}</strong></div>
        <div class="stat-card"><span class="muted">${escapeHtml(d.selected_month || this.state.dashboardFilter.month)} 问题记录</span><strong>${d.monthly_issue_num || 0}</strong></div>
        <div class="stat-card"><span class="muted">待补资料项目</span><strong>${d.missing_document_num || 0}</strong></div>
      </div>
      <div class="detail-grid">
        <div class="overview-card">
          <h3>现场版本分布</h3>
          ${!d.version_stats?.length ? `<div class="muted">暂无版本数据</div>` : `
            <div class="table-wrap">
              <table>
                <thead>
                  <tr><th>当前版本</th><th>项目数</th></tr>
                </thead>
                <tbody>
                  ${(d.version_stats || []).map(item => `
                    <tr>
                      <td><div class="ellipsis" title="${escapeAttr(item.current_version)}">${escapeHtml(item.current_version)}</div></td>
                      <td>${item.project_count}</td>
                    </tr>
                  `).join('')}
                </tbody>
              </table>
            </div>
          `}
        </div>
        <div class="overview-card">
          <h3>问题版本分布</h3>
          ${!d.issue_version_stats?.length ? `<div class="muted">暂无问题版本数据</div>` : `
            <div class="table-wrap">
              <table>
                <thead>
                  <tr><th>问题发生版本</th><th>问题数</th></tr>
                </thead>
                <tbody>
                  ${(d.issue_version_stats || []).map(item => `
                    <tr>
                      <td><div class="ellipsis" title="${escapeAttr(item.issue_version)}">${escapeHtml(item.issue_version)}</div></td>
                      <td>${item.issue_count}</td>
                    </tr>
                  `).join('')}
                </tbody>
              </table>
            </div>
          `}
        </div>
      </div>
    `;
  },

  renderIssueSummaryView() {
    const rows = this.state.issueSummary.list || [];
    return `
      ${this.renderPageIntro('问题汇总', '项目档案中登记的问题记录会自动汇总到这里，便于跨项目检索、排查和回看。')}
      <form class="filters" onsubmit="App.submitIssueFilter(event)">
        <div class="field">
          <label>关键字</label>
          <input value="${escapeAttr(this.state.issueFilter.keyword)}" oninput="App.updateIssueFilter('keyword', this.value)" placeholder="问题标题 / 项目名称 / 客户名称" />
        </div>
        <div class="field">
          <label>问题版本</label>
          <input value="${escapeAttr(this.state.issueFilter.issue_version)}" oninput="App.updateIssueFilter('issue_version', this.value)" placeholder="可按问题发生版本筛选" />
        </div>
        <div class="field field-actions">
          <button class="btn btn-primary" type="submit">查询</button>
          <button class="btn btn-secondary" type="button" onclick="App.resetIssueFilter()">重置</button>
        </div>
      </form>
      ${!rows.length ? `<div class="empty-card">暂无问题记录</div>` : `
        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th>项目名称</th>
                <th>客户名称</th>
                <th>问题标题</th>
                <th>发生版本</th>
                <th>发生方式</th>
                <th>问题时间</th>
                <th>操作人</th>
                <th>处理结果</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              ${rows.map(item => `
                <tr>
                  <td><button class="table-link ellipsis" type="button" title="${escapeAttr(item.project_name || '-')}" onclick="App.openIssueSummaryProject(${item.project_id})">${escapeHtml(item.project_name || '-')}</button></td>
                  <td><div class="ellipsis" title="${escapeAttr(item.customer_name || '-')}">${escapeHtml(item.customer_name || '-')}</div></td>
                  <td><div class="ellipsis" title="${escapeAttr(item.summary || '-')}">${escapeHtml(item.summary || '-')}</div></td>
                  <td><div class="ellipsis" title="${escapeAttr(item.issue_version || '-')}">${escapeHtml(item.issue_version || '-')}</div></td>
                  <td>${escapeHtml(this.displayServiceMode(item.service_mode))}</td>
                  <td>${formatDateTime(item.service_date)}</td>
                  <td><div class="ellipsis" title="${escapeAttr(item.owner_name || '-')}">${escapeHtml(item.owner_name || '-')}</div></td>
                  <td><div class="ellipsis" title="${escapeAttr(item.result_desc || item.problem_desc || '-')}">${escapeHtml(item.result_desc || item.problem_desc || '-')}</div></td>
                  <td>
                    <div class="inline-actions">
                      <button class="btn btn-secondary btn-inline" type="button" onclick="App.openIssueSummaryProject(${item.project_id})">查看项目</button>
                    </div>
                  </td>
                </tr>
              `).join('')}
            </tbody>
          </table>
        </div>
      `}
    `;
  },

  renderProjectTable() {
    const rows = this.state.projects.list || [];
    if (!rows.length) {
      return `<div class="empty-card">当前没有项目记录</div>`;
    }
    return `
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>项目名称</th>
              <th>客户名称</th>
              <th>当前版本</th>
              <th>状态</th>
              <th>最近升级</th>
              <th>最近变更</th>
              <th>更新时间</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            ${rows.map(item => `
              <tr>
                <td><button class="table-link ellipsis" type="button" title="${escapeAttr(item.project_name)}" onclick="App.openProjectDetail(${item.id})">${escapeHtml(item.project_name)}</button></td>
                <td><div class="ellipsis" title="${escapeAttr(item.customer_name)}">${escapeHtml(item.customer_name)}</div></td>
                <td>${escapeHtml(item.current_version || '-')}</td>
                <td>${this.renderStatusTag(item.project_status)}</td>
                <td>${formatDateTime(item.last_upgrade_at)}</td>
                <td>${formatDateTime(item.last_change_at)}</td>
                <td>${formatDateTime(item.updated_at)}</td>
                <td>
                  <div class="inline-actions">
                    <button class="btn btn-secondary btn-inline" type="button" onclick="App.openProjectDetail(${item.id})">查看</button>
                    ${this.canEdit() ? `<button class="btn btn-secondary btn-inline" type="button" onclick='App.openProjectForm(${jsonString(item)})'>编辑</button>` : ''}
                    ${this.canEdit() ? `<button class="btn btn-danger btn-inline" type="button" onclick="App.deleteProject(${item.id})">删除</button>` : ''}
                  </div>
                </td>
              </tr>
            `).join('')}
          </tbody>
        </table>
      </div>
    `;
  },

  renderProjectDetail() {
    const project = this.state.selectedProject;
    return `
      <div class="toolbar">
        <div>
          <button class="btn btn-secondary" type="button" onclick="App.backToProjects()">返回列表</button>
        </div>
      </div>
      <div class="project-header-card">
        <div class="project-head">
          <div>
            <h2 class="section-title section-title-tight detail-title">${escapeHtml(project.project_name)}</h2>
            ${this.state.projectLoading ? `<div class="muted inline-loading">正在加载项目详情...</div>` : ''}
            <div class="inline-actions">
              ${this.renderStatusTag(project.project_status)}
              <span class="tag">当前版本 ${escapeHtml(project.current_version || '-')}</span>
              <span class="tag">客户 ${escapeHtml(project.customer_name)}</span>
            </div>
          </div>
          <div class="inline-actions">
            ${this.canEdit() ? `<button class="btn btn-secondary" type="button" onclick='App.openProjectForm(${jsonString(project)})'>编辑项目</button>` : ''}
            ${this.canEdit() ? `<button class="btn btn-primary" type="button" onclick="App.openRecordForm('upgrades')">新增升级</button>` : ''}
            ${this.canEdit() ? `<button class="btn btn-secondary" type="button" onclick="App.openRecordForm('services')">新增服务</button>` : ''}
            ${this.canEdit() ? `<button class="btn btn-secondary" type="button" onclick="App.openRecordForm('issues')">新增问题</button>` : ''}
            ${this.canEdit() ? `<button class="btn btn-secondary" type="button" onclick="App.openAttachmentForm()">上传资料</button>` : ''}
            ${this.canEdit() ? `<button class="btn btn-danger" type="button" onclick="App.deleteProject(${project.id})">删除项目</button>` : ''}
          </div>
        </div>
        <div class="project-meta">
          <div><strong>项目编号：</strong>${escapeHtml(project.project_code || '-')}</div>
          <div><strong>实施日期：</strong>${escapeHtml(project.implementation_date || '-')}</div>
          <div><strong>上线日期：</strong>${escapeHtml(project.online_date || '')}</div>
          <div><strong>验收日期：</strong>${escapeHtml(project.acceptance_date || '')}</div>
          <div><strong>负责人：</strong>${escapeHtml(project.owner_name || (project.owner_user_id ? `#${project.owner_user_id}` : '-'))}</div>
          <div><strong>部署环境：</strong>${escapeHtml(this.displayDeployMode(project.deploy_mode))}</div>
          <div><strong>最近升级：</strong>${formatDateTime(project.last_upgrade_at)}</div>
          <div><strong>最近变更：</strong>${formatDateTime(project.last_change_at)}</div>
          <div><strong>客户联系人：</strong>${escapeHtml(project.customer_contact || '-')}</div>
          <div><strong>环境说明：</strong>${escapeHtml(project.environment_summary || '-')}</div>
        </div>
      </div>
      <div class="tab-bar">
        ${this.renderTabButton('overview', '概览')}
        ${this.renderTabButton('changes', '变更中心')}
        ${this.renderTabButton('services', '服务与问题')}
        ${this.renderTabButton('integrations', '外部对接')}
        ${this.renderTabButton('attachments', '文档资料')}
        ${this.renderTabButton('audit', '操作日志')}
      </div>
      <div class="tab-panel">
        ${this.renderProjectTabPanel()}
      </div>
    `;
  },

  renderProjectTabPanel() {
    if (this.state.tabLoading) {
      return `<div class="empty-card">正在加载...</div>`;
    }
    if (this.state.projectTab === 'overview') {
      return this.renderOverviewPanel();
    }
    if (this.state.projectTab === 'changes') {
      return this.renderChangePanel();
    }
    if (this.state.projectTab === 'services') {
      return this.renderServicePanel();
    }
    if (this.state.projectTab === 'integrations') {
      return this.renderIntegrationPanel();
    }
    if (this.state.projectTab === 'attachments') {
      return this.renderAttachmentPanel();
    }
    return this.renderAuditPanel();
  },

  renderOverviewPanel() {
    const data = this.state.projectOverview;
    if (!data) return `<div class="empty-card">正在加载项目概览...</div>`;
    const serviceItems = (data.recent_services || []).filter(item => item.service_type !== 'incident');
    const issueItems = (data.recent_services || []).filter(item => item.service_type === 'incident');
    return `
      <div class="detail-grid">
        <div class="overview-card">
          <h3>最近升级</h3>
          ${this.renderPlainList((data.recent_upgrades || []).map(item => `${item.source_version} -> ${item.target_version} / ${formatDateTime(item.upgrade_date)}`), '暂无升级记录')}
        </div>
        <div class="overview-card">
          <h3>最近配置变更</h3>
          ${this.renderPlainList((data.recent_config_changes || []).map(item => `${item.config_path} / ${item.effective_version || '-'}`), '暂无配置变更')}
        </div>
        <div class="overview-card">
          <h3>最近 SQL 变更</h3>
          ${this.renderPlainList((data.recent_sql_changes || []).map(item => `${item.change_title} / ${item.effective_version || '-'}`), '暂无 SQL 变更')}
        </div>
        <div class="overview-card">
          <h3>最近脚本资产</h3>
          ${this.renderPlainList((data.recent_assets || []).map(item => `${item.asset_name} / ${item.asset_type}`), '暂无脚本资产')}
        </div>
        <div class="overview-card">
          <h3>最近服务记录</h3>
          ${this.renderPlainList(serviceItems.map(item => `${item.summary} / ${formatDateTime(item.service_date)}`), '暂无服务记录')}
        </div>
        <div class="overview-card">
          <h3>最近问题记录</h3>
          ${this.renderPlainList(issueItems.map(item => `${item.summary} / ${item.issue_version || '未填版本'} / ${formatDateTime(item.service_date)}`), '暂无问题记录')}
        </div>
        <div class="overview-card">
          <h3>外部对接摘要</h3>
          ${this.renderPlainList((data.integrations || []).slice(0, 6).map(item => `${item.external_system_name} / ${item.integration_type} / ${item.joint_status}`), '暂无外部对接')}
        </div>
      </div>
    `;
  },

  renderChangePanel() {
    return `
      <div class="subtab-bar">
        ${this.renderChangeTabButton('upgrades', '升级记录')}
        ${this.renderChangeTabButton('configs', '配置变更')}
        ${this.renderChangeTabButton('sqls', 'SQL 变更')}
        ${this.renderChangeTabButton('assets', '脚本/补丁')}
      </div>
      ${this.canEdit() ? `<div class="toolbar toolbar-compact"><div></div><button class="btn btn-primary" type="button" onclick="App.openRecordForm('${this.state.changeTab}')">新增记录</button></div>` : ''}
      ${this.renderCurrentChangeTable()}
    `;
  },

  renderServicePanel() {
    const activeType = this.state.serviceTab === 'issues' ? 'issues' : 'services';
    return `
      <div class="subtab-bar">
        <button class="subtab-button ${activeType === 'services' ? 'active' : ''}" type="button" onclick="App.switchServiceTab('services')">服务记录</button>
        <button class="subtab-button ${activeType === 'issues' ? 'active' : ''}" type="button" onclick="App.switchServiceTab('issues')">问题记录</button>
      </div>
      ${this.canEdit() ? `<div class="toolbar toolbar-compact"><div></div><button class="btn btn-primary" type="button" onclick="App.openRecordForm('${activeType}')">新增记录</button></div>` : ''}
      ${this.renderServiceTable(activeType)}
    `;
  },

  renderServiceTable(type) {
    const rows = (this.state.records.serviceRecords || []).filter(item => type === 'issues' ? item.service_type === 'incident' : item.service_type !== 'incident');
    if (!rows.length) {
      return `<div class="empty-card">暂无数据</div>`;
    }
    const versionHead = type === 'issues' ? '<th>发生版本</th>' : '';
    const versionCell = item => type === 'issues'
      ? `<td><div class="ellipsis" title="${escapeAttr(item.issue_version || '-')}">${escapeHtml(item.issue_version || '-')}</div></td>`
      : '';
    return `
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>记录编号</th>
              <th>${type === 'issues' ? '问题标题' : '服务主题'}</th>
              <th>${type === 'issues' ? '问题类型' : '服务类型'}</th>
              <th>服务方式</th>
              ${versionHead}
              <th>时间</th>
              <th>操作人</th>
              <th>${type === 'issues' ? '处理结果' : '处理摘要'}</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            ${rows.map(item => `
              <tr>
                <td><div class="ellipsis" title="${escapeAttr(item.service_no)}">${escapeHtml(item.service_no)}</div></td>
                <td><div class="ellipsis" title="${escapeAttr(item.summary)}">${escapeHtml(item.summary)}</div></td>
                <td>${escapeHtml(this.displayServiceType(item.service_type))}</td>
                <td>${escapeHtml(this.displayServiceMode(item.service_mode))}</td>
                ${versionCell(item)}
                <td>${formatDateTime(item.service_date)}</td>
                <td><div class="ellipsis" title="${escapeAttr(item.owner_name || '-')}">${escapeHtml(item.owner_name || '-')}</div></td>
                <td><div class="ellipsis" title="${escapeAttr(item.result_desc || item.problem_desc || '-')}">${escapeHtml(item.result_desc || item.problem_desc || '-')}</div></td>
                <td>
                  <div class="inline-actions">
                    <button class="btn btn-secondary btn-inline" type="button" onclick='App.openRecordForm("${type}", ${jsonString(item)})'>编辑</button>
                    <button class="btn btn-danger btn-inline" type="button" onclick="App.deleteRecord('${type}', ${item.id})">删除</button>
                  </div>
                </td>
              </tr>
            `).join('')}
          </tbody>
        </table>
      </div>
    `;
  },

  renderCurrentChangeTable() {
    const tab = this.state.changeTab;
    if (tab === 'upgrades') {
      return this.renderGenericTable(this.state.records.upgrades, [
        ['升级编号', 'upgrade_no'],
        ['原版本', 'source_version'],
        ['目标版本', 'target_version'],
        ['状态', 'upgrade_status'],
        ['升级时间', 'upgrade_date'],
        ['操作人', 'owner_name'],
      ], 'upgrades');
    }
    if (tab === 'configs') {
      return this.renderGenericTable(this.state.records.configs, [
        ['记录编号', 'config_no'],
        ['配置路径', 'config_path'],
        ['生效版本', 'effective_version'],
        ['修改时间', 'changed_at'],
        ['操作人', 'changed_by_name'],
        ['备注', 'remark_text'],
      ], 'configs');
    }
    if (tab === 'sqls') {
      return this.renderGenericTable(this.state.records.sqls, [
        ['记录编号', 'sql_no'],
        ['修改标题', 'change_title'],
        ['数据库对象', 'db_objects'],
        ['修改时间', 'changed_at'],
        ['操作人', 'changed_by_name'],
        ['备注', 'remark_text'],
      ], 'sqls');
    }
    return this.renderGenericTable(this.state.records.assets, [
      ['记录编号', 'asset_no'],
      ['名称', 'asset_name'],
      ['类型', 'asset_type'],
      ['部署位置', 'deploy_path'],
      ['修改时间', 'changed_at'],
      ['操作人', 'changed_by_name'],
    ], 'assets');
  },

  renderIntegrationPanel() {
    return `
      ${this.canEdit() ? `<div class="toolbar toolbar-compact"><div></div><button class="btn btn-primary" type="button" onclick="App.openRecordForm('integrations')">新增对接</button></div>` : ''}
      ${this.renderGenericTable(this.state.records.integrations, [
        ['记录编号', 'integration_no'],
        ['对接系统', 'external_system_name'],
        ['对接方式', 'integration_type'],
        ['联调状态', 'joint_status'],
        ['备注', 'remark_text'],
      ], 'integrations')}
    `;
  },

  renderAttachmentPanel() {
    const items = this.state.records.attachments || [];
    return `
      <div class="attachment-toolbar">
        ${this.canEdit() ? `<button class="btn btn-primary" type="button" onclick="App.openAttachmentForm()">上传附件</button>` : ''}
      </div>
      ${!items.length ? `<div class="empty-card">暂无附件资料</div>` : `
        <div class="file-card-grid">
          ${items.map(item => `
            <div class="file-card">
              ${this.renderAttachmentThumb(item)}
              <h4 class="file-card-title">${escapeHtml(item.title)}</h4>
              <div class="muted file-card-meta">${escapeHtml(this.displayDocCategory(item.doc_category))} · ${formatFileSize(item.file_size)}</div>
              <p class="muted file-card-desc">${escapeHtml(item.description || item.original_name || '-')}</p>
              <div class="inline-actions">
                <button class="btn btn-secondary btn-inline" type="button" onclick="App.previewAttachment(${item.id})">预览</button>
                <button class="btn btn-secondary btn-inline" type="button" onclick="App.downloadAttachment(${item.id})">下载</button>
                ${this.canEdit() ? `<button class="btn btn-danger btn-inline" type="button" onclick="App.deleteAttachment(${item.id})">删除</button>` : ''}
              </div>
            </div>
          `).join('')}
        </div>
      `}
    `;
  },

  renderAttachmentThumb(item) {
    if (item.mime_type && item.mime_type.startsWith('image/')) {
      return `<img class="file-thumb file-thumb-image" src="${this.apiBase}/attachments/${item.id}/preview?access_token=${encodeURIComponent(this.state.token)}" alt="${escapeAttr(item.title)}" onerror="this.style.display='none'" />`;
    }
    return `
      <div class="file-thumb file-thumb-generic">
        <span class="file-thumb-ext">${escapeHtml(item.file_ext || 'file').toUpperCase()}</span>
      </div>
    `;
  },

  renderAuditPanel() {
    return this.renderGenericTable(this.state.records.auditLogs, [
      ['时间', 'operated_at'],
      ['对象类型', 'object_type'],
      ['操作类型', 'operation_type'],
      ['摘要', 'operation_summary'],
      ['操作人', 'operator_user_name'],
    ], 'audit', false);
  },

  renderGenericTable(rows, columns, type, withActions = true) {
    if (!rows || !rows.length) {
      return `<div class="empty-card">暂无数据</div>`;
    }
    return `
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              ${columns.map(([title]) => `<th>${escapeHtml(title)}</th>`).join('')}
              ${withActions ? '<th>操作</th>' : ''}
            </tr>
          </thead>
          <tbody>
            ${rows.map(item => `
              <tr>
                ${columns.map(([, key]) => `<td><div class="ellipsis" title="${escapeAttr(this.displayColumnValue(key, item[key], item))}">${escapeHtml(this.displayColumnValue(key, item[key], item))}</div></td>`).join('')}
                ${withActions ? `<td><div class="inline-actions">
                  <button class="btn btn-secondary btn-inline" type="button" onclick='App.openRecordForm("${type}", ${jsonString(item)})'>编辑</button>
                  <button class="btn btn-danger btn-inline" type="button" onclick="App.deleteRecord('${type}', ${item.id})">删除</button>
                </div></td>` : ''}
              </tr>
            `).join('')}
          </tbody>
        </table>
      </div>
    `;
  },

  renderUsersView() {
    return `
      ${this.renderPageIntro('用户管理', '统一维护内部账号，可新增、编辑、重置密码和停用账号。', `<button class="btn btn-primary" type="button" onclick="App.openUserForm()">新增用户</button>`)}
      ${!this.state.users.length ? `<div class="empty-card">暂无用户数据</div>` : `
        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th>账号</th>
                <th>真实姓名</th>
                <th>状态</th>
                <th>创建时间</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              ${this.state.users.map(user => `
                <tr>
                  <td>${escapeHtml(user.username)}</td>
                  <td>${escapeHtml(user.real_name)}</td>
                  <td>${user.status ? '<span class="tag">启用</span>' : '<span class="tag danger">停用</span>'}</td>
                  <td>${formatDateTime(user.created_at)}</td>
                  <td>
                    <div class="inline-actions">
                      <button class="btn btn-secondary btn-inline" type="button" onclick='App.openUserForm(${jsonString(user)})'>编辑</button>
                      ${this.isProtectedUser(user)
                        ? '<button class="btn btn-danger btn-inline" type="button" disabled title="admin 管理员账号不允许删除">删除</button>'
                        : `<button class="btn btn-danger btn-inline" type="button" onclick="App.deleteUser(${user.id})">删除</button>`}
                    </div>
                  </td>
                </tr>
              `).join('')}
            </tbody>
          </table>
        </div>
      `}
    `;
  },

  async deleteUser(id) {
    const user = this.state.users.find(item => item.id === id);
    if (this.isProtectedUser(user)) {
      this.showToast('admin 管理员账号不允许删除');
      return;
    }
    if (!confirm('确认删除此用户吗？')) return;
    try {
      await this.api(`/users/${id}`, { method: 'DELETE' });
      this.showToast('用户已删除');
      await this.loadUsers();
      this.render();
    } catch (error) {
      this.showToast(error.message);
    }
  },

  async deleteProject(id) {
    if (!confirm('确认删除该项目及其关联记录吗？删除后不可恢复。')) return;
    try {
      await this.api(`/projects/${id}`, { method: 'DELETE' });
      if (this.state.selectedProject?.id === id) {
        this.state.selectedProject = null;
        this.state.projectOverview = null;
        this.state.projectTab = 'overview';
        this.state.changeTab = 'upgrades';
      }
      this.showToast('项目已删除');
      await Promise.all([this.loadProjects(), this.loadDashboard()]);
      this.render();
    } catch (error) {
      this.showToast(error.message);
    }
  },

  renderTabButton(tab, label) {
    return `<button class="tab-button ${this.state.projectTab === tab ? 'active' : ''}" type="button" onclick="App.switchProjectTab('${tab}')">${label}</button>`;
  },

  renderChangeTabButton(tab, label) {
    return `<button class="subtab-button ${this.state.changeTab === tab ? 'active' : ''}" type="button" onclick="App.switchChangeTab('${tab}')">${label}</button>`;
  },

  renderPlainList(items, emptyText) {
    if (!items.length) {
      return `<div class="muted">${emptyText}</div>`;
    }
    return `<ul class="list-plain">${items.map(item => `<li>${escapeHtml(item)}</li>`).join('')}</ul>`;
  },

  renderStatusTag(status) {
    const mapping = {
      implementing: ['实施中', 'warning'],
      online: ['已上线', ''],
      maintenance: ['维护中', 'warning'],
      archived: ['已归档', 'danger'],
    };
    const [label, className] = mapping[status] || [status || '-', ''];
    return `<span class="tag ${className}">${label}</span>`;
  },

  renderModal() {
    const modal = this.state.modal;
    if (!modal) return '';
    if (modal.type === 'attachment') {
      return `
        <div class="modal-mask">
          <div class="modal" onclick="event.stopPropagation()">
            <div class="modal-header">
              <strong>上传附件</strong>
              <button type="button" class="modal-close" aria-label="关闭" onclick="App.closeModal()">×</button>
            </div>
            <form class="modal-form" onsubmit="App.submitAttachment(event)">
              <div class="modal-body grid-2">
                <div class="field"><label class="required">文档标题</label><input name="title" required /></div>
                <div class="field"><label class="required">分类</label><select name="doc_category">${this.renderOptions(this.docCategoryOptions(), 'other')}</select></div>
                <div class="field"><label>关联模块</label><select name="ref_type">${this.renderOptions([{ value: 'project', label: '项目' }, { value: 'upgrade', label: '升级记录' }, { value: 'config', label: '配置变更' }, { value: 'sql', label: 'SQL 变更' }, { value: 'integration', label: '外部对接' }, { value: 'asset', label: '脚本资产' }, { value: 'service', label: '服务/问题记录' }], 'project')}</select></div>
                <div class="field"><label>关联记录ID</label><input name="ref_id" type="number" placeholder="可选" /></div>
                <div class="field"><label class="required">文件</label><input name="file" type="file" required /></div>
                <div class="field"><label>标签</label><input name="tags" placeholder="逗号分隔" /></div>
                <div class="field field-span-full"><label>说明</label><textarea name="description"></textarea></div>
              </div>
              <div class="modal-footer">
                <button type="button" class="btn btn-secondary" onclick="App.closeModal()">取消</button>
                <button type="submit" class="btn btn-primary">开始上传</button>
              </div>
            </form>
          </div>
        </div>
      `;
    }
    if (modal.type === 'form') {
      return `
        <div class="modal-mask">
          <div class="modal" onclick="event.stopPropagation()">
            <div class="modal-header">
              <strong>${escapeHtml(modal.title)}</strong>
              <button type="button" class="modal-close" aria-label="关闭" onclick="App.closeModal()">×</button>
            </div>
            <form class="modal-form" onsubmit="App.submitModalForm(event)">
              <div class="modal-body ${modal.fields.some(field => field.type === 'textarea') ? 'grid-1' : 'grid-2'}">
                ${modal.fields.map(field => this.renderField(field)).join('')}
              </div>
              <div class="modal-footer">
                <button type="button" class="btn btn-secondary" onclick="App.closeModal()">取消</button>
                <button type="submit" class="btn btn-primary">保存</button>
              </div>
            </form>
          </div>
        </div>
      `;
    }
    return '';
  },

  renderField(field) {
    const modal = this.state.modal;
    const classes = field.required ? 'required' : '';
    const disabled = field.disabled ? 'disabled' : '';
    const hasDraft = modal?.type === 'form'
      && modal.formValues
      && Object.prototype.hasOwnProperty.call(modal.formValues, field.name);
    const value = hasDraft ? modal.formValues[field.name] : this.formatFieldValue(field.type, field.value ?? '');
    const placeholder = field.placeholder ? `placeholder="${escapeAttr(field.placeholder)}"` : '';
    if (field.type === 'hidden') {
      return `<input type="hidden" name="${field.name}" value="${escapeAttr(this.formatFieldValue(field.type, value))}" />`;
    }
    if (field.type === 'textarea') {
      const isRemarkField = field.name === 'remark_text';
      const textareaPlaceholder = field.placeholder || (isRemarkField ? '支持直接粘贴截图，保存后自动归档到附件中' : '');
      return `<div class="field"><label class="${classes}">${escapeHtml(field.label)}</label><textarea name="${field.name}" ${field.required ? 'required' : ''} ${textareaPlaceholder ? `placeholder="${escapeAttr(textareaPlaceholder)}"` : ''} oninput="App.updateModalDraft('${field.name}', this.value)" ${isRemarkField ? 'onpaste="App.handleRemarkPaste(event)"' : ''}>${escapeHtml(value)}</textarea>${isRemarkField ? this.renderRemarkScreenshotPanel() : ''}</div>`;
    }
    if (field.type === 'select') {
      return `<div class="field"><label class="${classes}">${escapeHtml(field.label)}</label><select name="${field.name}" ${field.required ? 'required' : ''} onchange="App.updateModalDraft('${field.name}', this.value)">${this.renderOptions(field.options || [], String(value))}</select></div>`;
    }
    if (field.type === 'date') {
      const inputClasses = ['date-input'];
      if (!value) {
        inputClasses.push('is-empty');
      }
      return `<div class="field"><label class="${classes}">${escapeHtml(field.label)}</label><input class="${inputClasses.join(' ')}" type="date" name="${field.name}" value="${escapeAttr(this.formatFieldValue(field.type, value))}" ${field.required ? 'required' : ''} ${placeholder} ${disabled} oninput="App.handleDateFieldInput(this, '${field.name}')" onchange="App.handleDateFieldInput(this, '${field.name}')" /></div>`;
    }
    return `<div class="field"><label class="${classes}">${escapeHtml(field.label)}</label><input type="${field.type || 'text'}" name="${field.name}" value="${escapeAttr(this.formatFieldValue(field.type, value))}" ${field.required ? 'required' : ''} ${placeholder} ${disabled} oninput="App.updateModalDraft('${field.name}', this.value)" /></div>`;
  },

  handleDateFieldInput(element, name) {
    this.updateModalDraft(name, element.value);
    if (element.value) {
      element.classList.remove('is-empty');
      return;
    }
    element.classList.add('is-empty');
  },

  renderRemarkScreenshotPanel() {
    const modal = this.state.modal;
    if (!modal || modal.type !== 'form') return '';
    const existing = modal.existingScreenshots || [];
    const pasted = modal.pastedImages || [];
    const token = encodeURIComponent(this.state.token || '');
    return `
        <div class="remark-image-panel">
          <div class="remark-image-tip">可在备注框中直接按 Ctrl+V 粘贴截图，保存后自动归档到附件中心。</div>
        ${modal.loadingScreenshots ? `<div class="muted text-small">正在加载已保存截图...</div>` : ''}
        ${existing.length ? `
          <div class="remark-image-section">
            <div class="remark-image-title">已保存截图</div>
            <div class="remark-image-grid">
              ${existing.map(item => `
                <div class="remark-image-card">
                  <img class="remark-image-thumb" src="${this.apiBase}/attachments/${item.id}/preview?access_token=${token}" alt="${escapeAttr(item.title || '截图')}" />
                  <div class="remark-image-actions">
                    <button type="button" class="btn btn-secondary btn-inline" onclick="App.previewAttachment(${item.id})">预览</button>
                    <button type="button" class="btn btn-secondary btn-inline" onclick="App.downloadAttachment(${item.id})">下载</button>
                  </div>
                </div>
              `).join('')}
            </div>
          </div>
        ` : ''}
        ${pasted.length ? `
          <div class="remark-image-section">
            <div class="remark-image-title">待上传截图</div>
            <div class="remark-image-grid">
              ${pasted.map(item => `
                <div class="remark-image-card">
                  <img class="remark-image-thumb" src="${item.previewUrl}" alt="${escapeAttr(item.fileName || '截图')}" />
                  <div class="remark-image-actions">
                    <button type="button" class="btn btn-danger btn-inline" onclick="App.removePastedImage('${item.id}')">移除</button>
                  </div>
                </div>
              `).join('')}
            </div>
          </div>
        ` : ''}
      </div>
    `;
  },

  formatFieldValue(type, value) {
    if (!value) return '';
    if (type === 'datetime-local') {
      return toDatetimeLocal(value);
    }
    return value;
  },

  renderOptions(options, current) {
    return options.map(option => `<option value="${escapeAttr(option.value)}" ${String(option.value) === String(current) ? 'selected' : ''}>${escapeHtml(option.label)}</option>`).join('');
  },

  displayValue(value) {
    if (value === null || value === undefined || value === '') return '-';
    if (typeof value === 'string' && value.includes('T') && value.includes(':')) {
      return formatDateTime(value);
    }
    return String(value);
  },

  displayColumnValue(key, value, item = {}) {
    if (key === 'upgrade_status') return this.displayUpgradeStatus(value);
    if (key === 'asset_type') return this.displayAssetType(value);
    if (key === 'integration_type') return this.displayIntegrationType(value);
    if (key === 'joint_status') return this.displayJointStatus(value);
    if (key === 'service_type') return this.displayServiceType(value);
    if (key === 'service_mode') return this.displayServiceMode(value);
    if (key === 'object_type') return this.displayAuditObjectType(value, item);
    if (key === 'operation_type') return this.displayAuditOperationType(value);
    return this.displayValue(value);
  },

  hasAdmin() {
    return !!this.state.currentUser;
  },

  isProtectedUser(user) {
    if (!user) return false;
    const username = String(user.username || '').trim().toLowerCase();
    const roles = Array.isArray(user.roles) ? user.roles.map(item => String(item || '').trim().toLowerCase()) : [];
    return username === 'admin' || roles.includes('admin');
  },

  canEdit() {
    return !!this.state.currentUser;
  },

  projectStatusOptions() {
    return [
      { value: 'implementing', label: '实施中' },
      { value: 'online', label: '已上线' },
      { value: 'maintenance', label: '维护中' },
      { value: 'archived', label: '已归档' },
    ];
  },

  deployModeOptions() {
    return [
      { value: 'standalone', label: '单机' },
      { value: 'cluster', label: '集群' },
    ];
  },

  displayDeployMode(value) {
    const mapping = {
      standalone: '单机',
      cluster: '集群',
      on_premise: '单机',
      cloud: '集群',
      hybrid: '集群',
    };
    return mapping[value] || value || '-';
  },

  displayUpgradeStatus(value) {
    const mapping = {
      planned: '计划中',
      completed: '已完成',
      rolled_back: '已回退',
    };
    return mapping[value] || value || '-';
  },

  displayAssetType(value) {
    const mapping = {
      script: '脚本',
      tool: '工具',
      patch: '补丁',
      temp_program: '临时程序',
    };
    return mapping[value] || value || '-';
  },

  displayIntegrationType(value) {
    const mapping = {
      api: '接口',
      database: '数据库',
      file: '文件',
      mq: '消息队列',
      manual: '人工导入导出',
    };
    return mapping[value] || value || '-';
  },

  displayJointStatus(value) {
    const mapping = {
      pending: '待联调',
      testing: '联调中',
      online: '已上线',
      disabled: '已停用',
    };
    return mapping[value] || value || '-';
  },

  displayAuditObjectType(value, item = {}) {
    if (item.object_type === 'service_record') {
      const snapshot = item.after_snapshot || item.before_snapshot || '';
      if (snapshot.includes('"service_type":"incident"')) {
        return '问题记录';
      }
      return '服务记录';
    }
    const mapping = {
      project: '项目档案',
      upgrade: '升级记录',
      config_change: '配置变更',
      sql_change: 'SQL变更',
      integration: '外部对接',
      asset: '脚本/补丁',
      attachment: '文档资料',
      user: '用户管理',
    };
    return mapping[value] || value || '-';
  },

  displayAuditOperationType(value) {
    const mapping = {
      create: '新增',
      update: '修改',
      delete: '删除',
      archive: '归档',
      reset_password: '重置密码',
    };
    return mapping[value] || value || '-';
  },

  displayServiceType(value) {
    const mapping = {
      implementation: '实施服务',
      inspection: '巡检服务',
      training: '培训服务',
      support: '日常支持',
      incident: '问题处理',
    };
    return mapping[value] || value || '-';
  },

  displayServiceMode(value) {
    const mapping = {
      onsite: '现场',
      remote: '远程',
    };
    return mapping[value] || value || '-';
  },

  displayDocCategory(value) {
    const mapping = {
      manual: '运维手册',
      deploy_doc: '部署说明',
      delivery_doc: '交付文档',
      arch_diagram: '架构图',
      deploy_diagram: '部署图',
      topology: '网络拓扑图',
      screenshot: '截图',
      other: '其他附件',
    };
    return mapping[value] || value || '-';
  },

  docCategoryOptions() {
    return [
      { value: 'manual', label: '运维手册' },
      { value: 'deploy_doc', label: '部署说明' },
      { value: 'delivery_doc', label: '交付文档' },
      { value: 'arch_diagram', label: '架构图' },
      { value: 'deploy_diagram', label: '部署图' },
      { value: 'topology', label: '网络拓扑图' },
      { value: 'other', label: '其他附件' },
    ];
  },

  recordFormConfig(type, record) {
    const commonDateTime = value => ({ type: 'datetime-local', value: value || new Date().toISOString() });
    const currentOperator = this.state.currentUser?.real_name || this.state.currentUser?.username || '';
    const operatorField = (value, label = '操作人') => ({ name: 'operator_display', label, value: value || currentOperator, disabled: true, submit: false });
    const map = {
      upgrades: {
        title: '升级记录',
        collectionEndpoint: '/upgrades',
        singleEndpoint: '/upgrades',
        fields: [
          operatorField(record?.owner_name, '负责人'),
          { name: 'upgrade_date', label: '升级日期', required: true, ...commonDateTime(record?.upgrade_date) },
          { name: 'source_version', label: '原版本', required: true, value: record?.source_version || '' },
          { name: 'target_version', label: '目标版本', required: true, value: record?.target_version || '' },
          {
            name: 'upgrade_status',
            label: '升级状态',
            type: 'select',
            required: true,
            value: record?.upgrade_status || 'completed',
            options: [
              { value: 'planned', label: '计划中' },
              { value: 'completed', label: '已完成' },
              { value: 'rolled_back', label: '已回退' },
            ],
          },
          {
            name: 'custom_retention',
            label: '定制保留',
            type: 'select',
            required: true,
            value: record?.custom_retention || 'partial',
            options: [
              { value: 'all', label: '全部' },
              { value: 'partial', label: '部分' },
              { value: 'none', label: '无' },
            ],
          },
          { name: 'issue_solution', label: '问题与方案', type: 'textarea', value: record?.issue_solution || '' },
          { name: 'test_result', label: '测试结果', type: 'textarea', value: record?.test_result || '' },
          { name: 'remark_text', label: '备注', type: 'textarea', value: record?.remark_text || '' },
        ],
      },
      configs: {
        title: '配置变更',
        collectionEndpoint: '/config-changes',
        singleEndpoint: '/config-changes',
        fields: [
          operatorField(record?.changed_by_name),
          { name: 'effective_version', label: '生效版本', value: record?.effective_version || '' },
          { name: 'config_path', label: '配置文件路径', required: true, value: record?.config_path || '' },
          { name: 'change_reason', label: '修改原因', required: true, value: record?.change_reason || '' },
          { name: 'changed_at', label: '修改时间', required: true, ...commonDateTime(record?.changed_at) },
          { name: 'before_content', label: '原配置内容', type: 'textarea', value: record?.before_content || '' },
          { name: 'after_content', label: '修改后配置内容', type: 'textarea', required: true, value: record?.after_content || '' },
          { name: 'test_result', label: '测试结果', type: 'textarea', value: record?.test_result || '' },
          { name: 'remark_text', label: '备注', type: 'textarea', value: record?.remark_text || '' },
        ],
      },
      sqls: {
        title: 'SQL 变更',
        collectionEndpoint: '/sql-changes',
        singleEndpoint: '/sql-changes',
        fields: [
          operatorField(record?.changed_by_name),
          { name: 'effective_version', label: '生效版本', value: record?.effective_version || '' },
          { name: 'change_title', label: '修改标题', required: true, value: record?.change_title || '' },
          { name: 'db_objects', label: '数据库对象', value: record?.db_objects || '' },
          { name: 'change_reason', label: '修改原因', required: true, value: record?.change_reason || '' },
          { name: 'changed_at', label: '修改时间', required: true, ...commonDateTime(record?.changed_at) },
          { name: 'change_sql', label: '变更 SQL', type: 'textarea', required: true, value: record?.change_sql || '' },
          { name: 'rollback_sql', label: '回退 SQL', type: 'textarea', value: record?.rollback_sql || '' },
          { name: 'test_result', label: '测试结果', type: 'textarea', value: record?.test_result || '' },
          { name: 'remark_text', label: '备注', type: 'textarea', value: record?.remark_text || '' },
        ],
      },
      integrations: {
        title: '外部对接',
        collectionEndpoint: '/integrations',
        singleEndpoint: '/integrations',
        fields: [
          { name: 'external_system_name', label: '对接系统名称', required: true, value: record?.external_system_name || '' },
          {
            name: 'integration_type',
            label: '对接方式',
            type: 'select',
            required: true,
            value: record?.integration_type || 'api',
            options: [
              { value: 'api', label: '接口' },
              { value: 'database', label: '数据库' },
              { value: 'file', label: '文件' },
              { value: 'mq', label: '消息队列' },
              { value: 'manual', label: '人工导入导出' },
            ],
          },
          {
            name: 'integration_direction',
            label: '对接方向',
            type: 'select',
            value: record?.integration_direction || 'bidirectional',
            options: [
              { value: 'inbound', label: '单向输入' },
              { value: 'outbound', label: '单向输出' },
              { value: 'bidirectional', label: '双向' },
            ],
          },
          {
            name: 'joint_status',
            label: '联调状态',
            type: 'select',
            required: true,
            value: record?.joint_status || 'pending',
            options: [
              { value: 'pending', label: '待联调' },
              { value: 'testing', label: '联调中' },
              { value: 'online', label: '已上线' },
              { value: 'disabled', label: '已停用' },
            ],
          },
          { name: 'external_owner', label: '外部负责人', value: record?.external_owner || '' },
          { name: 'endpoint_desc', label: '端点/对象说明', value: record?.endpoint_desc || '' },
          { name: 'content_desc', label: '对接内容说明', type: 'textarea', required: true, value: record?.content_desc || '' },
          { name: 'remark_text', label: '备注', type: 'textarea', value: record?.remark_text || '' },
        ],
      },
      services: {
        title: '服务记录',
        collectionEndpoint: '/service-records',
        singleEndpoint: '/service-records',
        fields: [
          operatorField(record?.owner_name),
          {
            name: 'service_type',
            label: '服务类型',
            type: 'select',
            required: true,
            value: record?.service_type || 'support',
            options: [
              { value: 'support', label: '日常支持' },
              { value: 'implementation', label: '实施服务' },
              { value: 'inspection', label: '巡检服务' },
              { value: 'training', label: '培训服务' },
            ],
          },
          { name: 'service_mode', label: '服务方式', type: 'select', value: record?.service_mode || 'remote', options: [{ value: 'remote', label: '远程' }, { value: 'onsite', label: '现场' }] },
          { name: 'service_date', label: '服务时间', required: true, ...commonDateTime(record?.service_date) },
          { name: 'summary', label: '服务主题', required: true, value: record?.summary || '' },
          { name: 'problem_desc', label: '现场情况', type: 'textarea', value: record?.problem_desc || '' },
          { name: 'process_desc', label: '处理过程', type: 'textarea', value: record?.process_desc || '' },
          { name: 'result_desc', label: '处理结果', type: 'textarea', value: record?.result_desc || '' },
          { name: 'next_action', label: '后续动作', type: 'textarea', value: record?.next_action || '' },
          { name: 'remark_text', label: '备注', type: 'textarea', value: record?.remark_text || '' },
        ],
      },
      issues: {
        title: '问题记录',
        collectionEndpoint: '/service-records',
        singleEndpoint: '/service-records',
        fields: [
          operatorField(record?.owner_name),
          { name: 'service_type', type: 'hidden', value: record?.service_type || 'incident' },
          { name: 'service_mode', label: '问题发生方式', type: 'select', value: record?.service_mode || 'onsite', options: [{ value: 'onsite', label: '现场' }, { value: 'remote', label: '远程' }] },
          { name: 'service_date', label: '问题时间', required: true, ...commonDateTime(record?.service_date) },
          { name: 'issue_version', label: '问题发生版本', value: record?.issue_version || '', placeholder: '非必填，用户使用不当导致的问题可留空' },
          { name: 'summary', label: '问题标题', required: true, value: record?.summary || '' },
          { name: 'problem_desc', label: '问题现象', type: 'textarea', required: true, value: record?.problem_desc || '' },
          { name: 'process_desc', label: '排查过程', type: 'textarea', value: record?.process_desc || '' },
          { name: 'result_desc', label: '处理结果', type: 'textarea', value: record?.result_desc || '' },
          { name: 'next_action', label: '后续动作', type: 'textarea', value: record?.next_action || '' },
          { name: 'remark_text', label: '备注', type: 'textarea', value: record?.remark_text || '' },
        ],
      },
      assets: {
        title: '脚本/补丁',
        collectionEndpoint: '/assets',
        singleEndpoint: '/assets',
        fields: [
          operatorField(record?.changed_by_name),
          { name: 'asset_name', label: '名称', required: true, value: record?.asset_name || '' },
          {
            name: 'asset_type',
            label: '类型',
            type: 'select',
            required: true,
            value: record?.asset_type || 'script',
            options: [
              { value: 'script', label: '脚本' },
              { value: 'tool', label: '工具' },
              { value: 'patch', label: '补丁' },
              { value: 'temp_program', label: '临时程序' },
            ],
          },
          { name: 'deploy_path', label: '部署位置', value: record?.deploy_path || '' },
          { name: 'purpose_desc', label: '用途说明', required: true, value: record?.purpose_desc || '' },
          { name: 'execute_command', label: '执行方式', type: 'textarea', value: record?.execute_command || '' },
          { name: 'rollback_method', label: '回退方式', type: 'textarea', value: record?.rollback_method || '' },
          { name: 'test_result', label: '测试结果', type: 'textarea', value: record?.test_result || '' },
          { name: 'changed_at', label: '修改时间', required: true, ...commonDateTime(record?.changed_at) },
          { name: 'remark_text', label: '备注', type: 'textarea', value: record?.remark_text || '' },
        ],
      },
    };
    return map[type];
  },
};

function escapeHtml(value) {
  return String(value ?? '')
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;');
}

function escapeAttr(value) {
  return escapeHtml(value).replaceAll("'", '&#39;');
}

function jsonString(value) {
  return escapeAttr(JSON.stringify(value));
}

function formatDateTime(value) {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return String(value);
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

function toDatetimeLocal(value) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

function pad(num) {
  return String(num).padStart(2, '0');
}

function todayDate() {
  const date = new Date();
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
}

function currentMonthValue() {
  const date = new Date();
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}`;
}

function formatFileSize(size) {
  if (!size) return '0 B';
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
  return `${(size / 1024 / 1024).toFixed(1)} MB`;
}

window.App = App;
window.addEventListener('DOMContentLoaded', () => App.init());

