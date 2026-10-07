param(
    [Parameter(Mandatory)][string]$Region,
    [Parameter(Mandatory)][string]$CertificateArn,
    [string]$Profile,
    [string]$StackName = 'johan-backend',
    [string]$ImageTag = (Get-Date -Format 'yyyyMMddHHmmss'),
    [string]$PostgresVersion = '17.11',
    [ValidateSet('disabled','openai','openai-compatible')][string]$AIProvider = 'disabled',
    [string]$AIModel = '',
    [string]$AIBaseURL = 'https://api.openai.com/v1'
)
$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '../..')
Get-Command aws,docker -ErrorAction Stop | Out-Null
$env:AWS_DEFAULT_REGION = $Region
if ($Profile) { $env:AWS_PROFILE = $Profile }
function Invoke-Aws {
    param([string[]]$Arguments)
    $result = & aws @Arguments --region $Region --output json
    if ($LASTEXITCODE -ne 0) { throw 'Operación AWS fallida; revisar permisos y región' }
    if ($result) { return ($result | ConvertFrom-Json) }
}
function Deploy-Stack {
    param([string]$Name,[string]$Template,[string[]]$Parameters)
    $arguments = @('cloudformation','deploy','--stack-name',$Name,'--template-file',$Template,'--capabilities','CAPABILITY_IAM','--no-fail-on-empty-changeset','--region',$Region)
    if ($Parameters.Count) { $arguments += @('--parameter-overrides') + $Parameters }
    & aws @arguments
    if ($LASTEXITCODE -ne 0) { throw "Stack fallido: $Name" }
}
function Stack-Outputs {
    param([string]$Name)
    $stack = Invoke-Aws -Arguments @('cloudformation','describe-stacks','--stack-name',$Name)
    $values = @{}
    foreach ($item in $stack.Stacks[0].Outputs) { $values[$item.OutputKey] = $item.OutputValue }
    return $values
}
function Run-Admin {
    param([hashtable]$Outputs,[string]$Command)
    $network = @{ awsvpcConfiguration = @{ subnets = $Outputs.AppSubnets.Split(','); securityGroups = @($Outputs.APISecurityGroup); assignPublicIp = 'DISABLED' } } | ConvertTo-Json -Depth 4 -Compress
    $overrides = @{ containerOverrides = @(@{ name = 'backend'; command = @($Command) }) } | ConvertTo-Json -Depth 4 -Compress
    $task = Invoke-Aws -Arguments @('ecs','run-task','--cluster',$Outputs.Cluster,'--task-definition',$Outputs.AdminTaskDefinition,'--launch-type','FARGATE','--network-configuration',$network,'--overrides',$overrides)
    if ($task.failures.Count -gt 0 -or $task.tasks.Count -ne 1) { throw 'No se pudo iniciar la tarea administrativa' }
    $arn = $task.tasks[0].taskArn
    & aws ecs wait tasks-stopped --cluster $Outputs.Cluster --tasks $arn --region $Region
    if ($LASTEXITCODE -ne 0) { throw 'La tarea administrativa no terminó a tiempo' }
    $result = Invoke-Aws -Arguments @('ecs','describe-tasks','--cluster',$Outputs.Cluster,'--tasks',$arn)
    if ($result.tasks[0].containers[0].exitCode -ne 0) { throw 'Tarea administrativa fallida; revisar CloudWatch antes de continuar' }
}
docker info --format '{{.OSType}}' | Out-Null
if ($LASTEXITCODE -ne 0) { throw 'Docker debe estar iniciado en modo Linux' }
if ($AIProvider -ne 'disabled' -and (-not $env:AI_API_KEY -or -not $AIModel)) { throw 'Configura AI_API_KEY localmente y especifica AIModel' }
$identity = Invoke-Aws -Arguments @('sts','get-caller-identity')
$engine = Invoke-Aws -Arguments @('rds','describe-db-engine-versions','--engine','postgres','--engine-version',$PostgresVersion)
if ($engine.DBEngineVersions.Count -eq 0) { throw 'Versión PostgreSQL no disponible en esta región' }
$certificate = Invoke-Aws -Arguments @('acm','describe-certificate','--certificate-arn',$CertificateArn)
if ($certificate.Certificate.Status -ne 'ISSUED') { throw 'El certificado ACM aún no está emitido' }
& aws cloudformation validate-template --template-body file://deploy/aws/backend.template.json --region $Region | Out-Null
if ($LASTEXITCODE -ne 0) { throw 'Plantilla AWS inválida' }
# Este procedimiento es para la primera instalación o para retomarla con DesiredCount=0.
$existing = & aws cloudformation describe-stacks --stack-name $StackName --region $Region --output json 2>$null
if ($LASTEXITCODE -eq 0) {
    $previous = $existing | ConvertFrom-Json
    $service = $previous.Stacks[0].Outputs | Where-Object OutputKey -eq 'Service'
    $cluster = $previous.Stacks[0].Outputs | Where-Object OutputKey -eq 'Cluster'
    if ($service -and $cluster) {
        $current = Invoke-Aws -Arguments @('ecs','describe-services','--cluster',$cluster.OutputValue,'--services',$service.OutputValue)
        if ($current.services[0].desiredCount -gt 0) { throw 'El servicio ya está activo; usa el procedimiento de actualización de docs/aws.md' }
    }
}
Deploy-Stack -Name "$StackName-ecr" -Template 'deploy/aws/ecr.template.json' -Parameters @()
$repository = (Stack-Outputs -Name "$StackName-ecr").RepositoryURI
$image = "${repository}:$ImageTag"
$registry = $repository.Split('/')[0]
& aws ecr get-login-password --region $Region | docker login --username AWS --password-stdin $registry
if ($LASTEXITCODE -ne 0) { throw 'Login ECR fallido' }
docker buildx build --platform linux/amd64 --tag $image --push .
if ($LASTEXITCODE -ne 0) { throw 'Build o push ECR fallido' }
$parameters = @("ImageURI=$image","CertificateArn=$CertificateArn","PostgresVersion=$PostgresVersion",'DesiredCount=0',"AIProvider=$AIProvider","AIModel=$AIModel","AIBaseURL=$AIBaseURL")
Deploy-Stack -Name $StackName -Template 'deploy/aws/backend.template.json' -Parameters $parameters
$outputs = Stack-Outputs -Name $StackName
if ($AIProvider -ne 'disabled') {
    $temporary = [System.IO.Path]::GetTempFileName()
    try {
        $payload = @{ api_key = $env:AI_API_KEY } | ConvertTo-Json -Compress
        [System.IO.File]::WriteAllText($temporary,$payload,[System.Text.UTF8Encoding]::new($false))
        Invoke-Aws -Arguments @('secretsmanager','put-secret-value','--secret-id',$outputs.AISecretArn,'--secret-string',"file://$temporary") | Out-Null
    } finally { Remove-Item -LiteralPath $temporary -Force }
}
Run-Admin -Outputs $outputs -Command 'exec /app/migrate up'
Run-Admin -Outputs $outputs -Command 'exec /app/provision'
$parameters = $parameters | Where-Object { $_ -ne 'DesiredCount=0' }
$parameters += 'DesiredCount=1'
Deploy-Stack -Name $StackName -Template 'deploy/aws/backend.template.json' -Parameters $parameters
& aws ecs wait services-stable --cluster $outputs.Cluster --services $outputs.Service --region $Region
if ($LASTEXITCODE -ne 0) { throw 'ECS no alcanzó un estado estable; revisar CloudWatch' }
Write-Output "Backend desplegado. Configura el dominio del certificado hacia $($outputs.ALBHost) y verifica /health y /ready por HTTPS."
Write-Output "CloudWatch: $($outputs.LogGroup)"
