# Sistema de Aprovação de Transferência de Arquivos

Este documento descreve o novo sistema de aprovação de transferência de arquivos implementado no cliente OnliDesk Qt.

## Visão Geral

O sistema de aprovação permite que os usuários controlem quais transferências de arquivos são permitidas, fornecendo:

- **Diálogo de aprovação interativo** com informações detalhadas sobre o arquivo e técnico
- **Configurações de segurança** para tipos de arquivo e tamanhos máximos
- **Opção de lembrar decisões** para evitar aprovações repetitivas
- **Avisos de segurança** para tipos de arquivo potencialmente perigosos
- **Timeout automático** para evitar diálogos pendentes indefinidamente

## Componentes Principais

### 1. ApprovalDialog

Classe responsável por exibir o diálogo de aprovação ao usuário.

**Características:**
- Interface moderna e intuitiva
- Exibição de informações do arquivo (nome, tamanho, tipo)
- Informações do técnico solicitante
- Avisos de segurança para arquivos perigosos
- Opção "Lembrar minha decisão"
- Timeout configurável
- Suporte a teclas de atalho (Enter/Escape)

### 2. FileTransferManager (Melhorado)

Gerenciador principal com funcionalidades de aprovação integradas.

**Novos Métodos:**
```cpp
// Configurações de aprovação
void setAutoApprovalEnabled(bool enabled);
void setApprovalTimeout(int seconds);
void setRememberDecisionEnabled(bool enabled);

// Configurações de segurança
void addAllowedFileExtension(const QString &extension);
void removeAllowedFileExtension(const QString &extension);
void setMaxFileSize(qint64 maxSize);
QStringList getAllowedFileExtensions() const;
```

**Novos Sinais:**
```cpp
// Eventos de aprovação
void transferApprovalRequested(const FileTransferRequest &request);
void transferApprovalDecision(const QString &transferId, bool approved, const QString &message);

// Eventos de segurança
void securityWarning(const QString &message, const QString &details);
void fileValidationFailed(const QString &filePath, const QString &reason);
void unauthorizedTransferAttempt(const QString &details);
```

## Como Usar

### 1. Configuração Básica

```cpp
#include "FileTransferManager.h"

// Criar instância
FileTransferManager *manager = new FileTransferManager(this);

// Configurar aprovação
manager->setAutoApprovalEnabled(false);  // Requer aprovação manual
manager->setApprovalTimeout(30);         // 30 segundos de timeout
manager->setRememberDecisionEnabled(true); // Lembrar decisões

// Configurar segurança
manager->setMaxFileSize(50 * 1024 * 1024); // 50MB máximo
manager->addAllowedFileExtension(".pdf");
manager->addAllowedFileExtension(".txt");
manager->addAllowedFileExtension(".jpg");
```

### 2. Conectar Sinais

```cpp
// Conectar eventos de aprovação
connect(manager, &FileTransferManager::transferApprovalRequested,
        this, &MyClass::onTransferApprovalRequested);
        
connect(manager, &FileTransferManager::transferApprovalDecision,
        this, &MyClass::onTransferApprovalDecision);
        
// Conectar eventos de segurança
connect(manager, &FileTransferManager::securityWarning,
        this, &MyClass::onSecurityWarning);
        
connect(manager, &FileTransferManager::fileValidationFailed,
        this, &MyClass::onFileValidationFailed);
```

### 3. Implementar Handlers

```cpp
void MyClass::onTransferApprovalRequested(const FileTransferRequest &request)
{
    qDebug() << "Aprovação solicitada para:" << request.filename;
    // O diálogo será exibido automaticamente
}

void MyClass::onTransferApprovalDecision(const QString &transferId, bool approved, const QString &message)
{
    if (approved) {
        qDebug() << "Transferência aprovada:" << transferId;
    } else {
        qDebug() << "Transferência rejeitada:" << transferId << "Motivo:" << message;
    }
}

void MyClass::onSecurityWarning(const QString &message, const QString &details)
{
    QMessageBox::warning(this, "Aviso de Segurança", message + "\n\n" + details);
}
```

## Configurações de Segurança

### Extensões de Arquivo Permitidas

Por padrão, as seguintes extensões são permitidas:
- Documentos: `.txt`, `.pdf`, `.doc`, `.docx`, `.xls`, `.xlsx`, `.ppt`, `.pptx`
- Imagens: `.jpg`, `.jpeg`, `.png`, `.gif`, `.bmp`, `.svg`
- Arquivos compactados: `.zip`, `.rar`, `.7z`
- Código: `.cpp`, `.h`, `.js`, `.html`, `.css`, `.py`, `.java`

### Extensões Perigosas

As seguintes extensões são consideradas perigosas e exibem avisos especiais:
- Executáveis: `.exe`, `.bat`, `.cmd`, `.com`, `.scr`, `.msi`
- Scripts: `.vbs`, `.js`, `.ps1`, `.sh`
- Outros: `.dll`, `.sys`, `.reg`

### Tamanho Máximo

O tamanho máximo padrão é de 100MB. Arquivos maiores são automaticamente rejeitados.

## Fluxo de Aprovação

1. **Solicitação Recebida**: Técnico solicita transferência de arquivo
2. **Validação Inicial**: Verificação de extensão e tamanho
3. **Verificação de Decisão Lembrada**: Se habilitado, verifica decisões anteriores
4. **Exibição do Diálogo**: Se necessário, exibe diálogo de aprovação
5. **Processamento da Decisão**: Aprova ou rejeita a transferência
6. **Salvamento da Decisão**: Se habilitado, salva a decisão para futuras referências

## Persistência de Configurações

As configurações são automaticamente salvas usando `QSettings`:

- **Organização**: "OnliDesk"
- **Aplicação**: "FileTransfer"
- **Localização**: Registro do Windows ou arquivo de configuração

### Chaves de Configuração

- `approval/autoEnabled`: Aprovação automática habilitada
- `approval/timeout`: Timeout em segundos
- `approval/rememberEnabled`: Lembrar decisões habilitado
- `security/allowedExtensions`: Lista de extensões permitidas
- `security/maxFileSize`: Tamanho máximo de arquivo
- `decisions/[hash]`: Decisões lembradas (hash do arquivo + técnico)

## Exemplo Completo

Veja o arquivo `examples/file_transfer_approval_example.cpp` para um exemplo completo de implementação.

## Considerações de Segurança

1. **Validação Dupla**: Tanto cliente quanto servidor devem validar arquivos
2. **Whitelist de Extensões**: Use lista de extensões permitidas, não bloqueadas
3. **Limite de Tamanho**: Defina limites apropriados para evitar ataques DoS
4. **Timeout**: Configure timeouts para evitar diálogos pendentes
5. **Logs de Auditoria**: Registre todas as decisões de aprovação
6. **Criptografia**: Use conexões seguras (WSS) para transferências

## Troubleshooting

### Diálogo não aparece
- Verifique se `autoApprovalEnabled` está `false`
- Confirme que não há decisão lembrada para o arquivo
- Verifique se o arquivo passa na validação inicial

### Transferência rejeitada automaticamente
- Verifique se a extensão está na lista de permitidas
- Confirme se o tamanho não excede o limite
- Verifique logs de segurança

### Configurações não persistem
- Verifique permissões de escrita no registro/arquivo de configuração
- Confirme que `QSettings` está configurado corretamente

## Futuras Melhorias

- Integração com antivírus para scan automático
- Suporte a assinaturas digitais
- Interface web para configuração remota
- Relatórios de auditoria detalhados
- Políticas de grupo para configuração centralizada