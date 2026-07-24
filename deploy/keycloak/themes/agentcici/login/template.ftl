<#import "field.ftl" as field>
<#import "footer.ftl" as loginFooter>
<#import "theme-resources.ftl" as themeResourceTags>

<#macro username>
  <#assign label><#if !realm.loginWithEmailAllowed>${msg("username")}<#elseif !realm.registrationEmailAsUsername>${msg("usernameOrEmail")}<#else>${msg("email")}</#if></#assign>
  <@field.group name="username" label=label>
    <div class="${properties.kcInputGroup}">
      <div class="${properties.kcInputGroupItemClass} ${properties.kcFill}"><span class="${properties.kcInputClass} ${properties.kcFormReadOnlyClass}"><input id="kc-attempted-username" value="${auth.attemptedUsername}" readonly></span></div>
      <div class="${properties.kcInputGroupItemClass}"><button id="reset-login" class="${properties.kcFormPasswordVisibilityButtonClass} kc-login-tooltip" type="button" aria-label="${msg('restartLoginTooltip')}" onclick="location.href='${url.loginRestartFlowUrl}'"><i class="fa-sync-alt fas" aria-hidden="true"></i><span class="kc-tooltip-text">${msg("restartLoginTooltip")}</span></button></div>
    </div>
  </@field.group>
</#macro>

<#macro registrationLayout bodyClass="" displayInfo=false displayMessage=true displayRequiredFields=false>
<!DOCTYPE html>
<html class="${properties.kcHtmlClass!}" lang="${lang}">
<head>
  <meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>${title!}</title>
  <#if themeResources?? && themeResources.favicons?has_content><@themeResourceTags.renderFavicons themeResources.favicons url.resourcesPath /><#else><link rel="icon" href="${url.resourcesPath}/img/favicon.ico" /></#if>
  <#if themeResources?? && themeResources.stylesCommon?has_content><@themeResourceTags.renderStyles themeResources.stylesCommon url.resourcesCommonPath /><#elseif properties.stylesCommon?has_content><#list properties.stylesCommon?split(' ') as style><link href="${url.resourcesCommonPath}/${style}" rel="stylesheet" /></#list></#if>
  <#if themeResources?? && themeResources.styles?has_content><@themeResourceTags.renderStyles themeResources.styles url.resourcesPath /><#elseif properties.styles?has_content><#list properties.styles?split(' ') as style><link href="${url.resourcesPath}/${style}" rel="stylesheet" /></#list></#if>
  <script type="importmap">{ "imports": { "rfc4648": "${url.resourcesCommonPath}/vendor/rfc4648/rfc4648.js" } }</script>
  <#if properties.scripts?has_content><#list properties.scripts?split(' ') as script><script src="${url.resourcesPath}/${script}" type="text/javascript"></script></#list></#if>
  <#if scripts??><#list scripts as script><script src="${script}" type="text/javascript"></script></#list></#if>
  <script type="module" src="${url.resourcesPath}/js/passwordVisibility.js"></script>
  <script type="module"><#outputformat "JavaScript">import { startSessionPolling } from ${(url.resourcesPath + "/js/authChecker.js")?c}; startSessionPolling(${url.ssoLoginInOtherTabsUrl?c});</#outputformat></script>
  <#if authenticationSession??><script type="module"><#outputformat "JavaScript">import { checkAuthSession } from ${(url.resourcesPath + "/js/authChecker.js")?c}; checkAuthSession(${authenticationSession.authSessionIdHash?c});</#outputformat></script></#if>
</head>
<body id="keycloak-bg" class="${properties.kcBodyClass!}" data-page-id="login-${pageId}">
  <main class="login-mode2">
    <section class="login-mode2__center">
      <section class="login-mode2__cube-zone" aria-label="思思能力立方体">
        <div class="login-mode2__cube-stage" aria-hidden="true"><div class="login-mode2__cube">
          <div class="login-mode2__cube-face login-mode2__cube-face--front"><img src="${url.resourcesPath}/img/cici-login-default.png" alt="" decoding="async" loading="eager" draggable="false"></div>
          <div class="login-mode2__cube-face login-mode2__cube-face--back is-contain"><img src="${url.resourcesPath}/img/login-cube-cloudcc.webp" alt="" decoding="async" loading="eager" draggable="false"></div>
          <div class="login-mode2__cube-face login-mode2__cube-face--right is-contain"><img src="${url.resourcesPath}/img/login-cube-openai.webp" alt="" decoding="async" loading="eager" draggable="false"></div>
          <div class="login-mode2__cube-face login-mode2__cube-face--left is-contain"><img src="${url.resourcesPath}/img/login-cube-deepseek.webp" alt="" decoding="async" loading="eager" draggable="false"></div>
          <div class="login-mode2__cube-face login-mode2__cube-face--top"><img src="${url.resourcesPath}/img/login-cube-ai-chip.webp" alt="" decoding="async" loading="eager" draggable="false"></div>
          <div class="login-mode2__cube-face login-mode2__cube-face--bottom is-contain"><img src="${url.resourcesPath}/img/login-cube-cloudcc.webp" alt="" decoding="async" loading="eager" draggable="false"></div>
        </div></div>
      </section>
      <section class="login-mode2__form-shell" aria-label="前台账号登录">
        <h1 class="keycloak-login-mode2__title" id="kc-page-title"><#nested "header"></h1>
        <p class="boot-login__notice">使用 AgentCiCi 统一账号登录。账号密码和多因素验证由统一身份中心安全处理。</p>
        <#if auth?has_content && auth.showUsername() && !auth.showResetCredentials()><div class="${properties.kcFormClass} ${properties.kcContentWrapperClass}"><#nested "show-username"><@username /></div></#if>
        <#if displayMessage && message?has_content && (message.type != 'warning' || !isAppInitiatedAction??)><div class="${properties.kcAlertClass!} pf-m-${(message.type = 'error')?then('danger', message.type)}"><div class="${properties.kcAlertIconClass!}"><#if message.type = 'success'><span class="${properties.kcFeedbackSuccessIcon!}"></span></#if><#if message.type = 'warning'><span class="${properties.kcFeedbackWarningIcon!}"></span></#if><#if message.type = 'error'><span class="${properties.kcFeedbackErrorIcon!}"></span></#if><#if message.type = 'info'><span class="${properties.kcFeedbackInfoIcon!}"></span></#if></div><span class="${properties.kcAlertTitleClass!} kc-feedback-text">${message.summary}</span></div></#if>
        <#nested "form">
        <#if auth?has_content && auth.showTryAnotherWayLink()><form id="kc-select-try-another-way-form" action="${url.loginAction}" method="post" novalidate="novalidate"><input type="hidden" name="tryAnotherWay" value="on"/><a id="try-another-way" href="javascript:document.forms['kc-select-try-another-way-form'].requestSubmit()" class="${properties.kcButtonSecondaryClass} ${properties.kcButtonBlockClass} ${properties.kcMarginTopClass}">${msg("doTryAnotherWay")}</a></form></#if>
        <#if switchOrganizationEnabled?? && switchOrganizationEnabled><form id="kc-switch-organization-form" action="${url.loginAction}" method="post" novalidate="novalidate"><input type="hidden" name="switchOrganization" value="true"/><a id="switch-organization" href="javascript:document.forms['kc-switch-organization-form'].requestSubmit()" class="${properties.kcButtonSecondaryClass} ${properties.kcButtonBlockClass} ${properties.kcMarginTopClass}">${msg("doSwitchOrganization")}</a></form></#if>
        <#nested "socialProviders">
        <#if displayInfo><div id="kc-info" class="${properties.kcLoginMainFooterBand!} ${properties.kcFormClass}"><#nested "info"></div></#if>
        <div class="${properties.kcLoginMainFooter!}"><@loginFooter.content/></div>
      </section>
    </section>
  </main>
</body>
</html>
</#macro>
